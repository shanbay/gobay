package seqgenext

/*
核心原理是根据时间戳的自增特性, 加上自增数解决统一毫秒下的并发问题
虽然时间戳是一个 64 位整数, 但实际远未达到该长度
使用 48 位存储毫秒时间, 自 BEGINNING_TIMESTAMP 后 2 ** 48 / 1000 / 3600 / 24 = 3257812.2304474073 天后此算法才会失效

"{:064b}".format(((int(time.time() * 1000) - BEGINNING_TIMESTAMP) << TIMESTAMP_SHIFT) + MAX_SEQUENCE)
'0000000010110101100111101101011000010111010001100000000000000000'
*/
import (
	"fmt"
	"sync"

	"github.com/go-redis/redis"
	"github.com/shanbay/gobay"
)

const (
	timestampShift = 15
	maxStep        = 1 << (timestampShift - 2)
	// 空一位是为了避免 incrby step 超出 14 位导致自增溢出
	maxSequence = 1 << (timestampShift - 2)
	// 起始时间的作用是避免时间过早的达到边界
	// 1514764800 是 UTC 时间 2018-01-01 00:00:00
	beginningTimestamp = 1514764800
	// 由于 increment 跟时间无关, 某一毫秒内 increment 可能出现 ... MAX_SEQUENCE, 1, 2, ... 的序列, sequence 时间递增精度只能为毫秒
	luaScript = `
local sq = redis.call('incrby', KEYS[1], ARGV[2])
if(sq>tonumber(ARGV[1]))
then
redis.call('del', KEYS[1])
end
local t =  redis.call('time')
return {sq, tonumber(t[1]), tonumber(t[2])}
`
)

type SequenceGeneratorExt struct {
	redisClient  *redis.Client
	app          *gobay.Application
	NS           string
	RedisExtName gobay.Key
	SequenceBase uint64
	SequenceKey  string
}

// Init implements Extension interface
func (d *SequenceGeneratorExt) Init(app *gobay.Application) error {
	config := app.Config()
	if d.NS != "" {
		config = config.Sub(d.NS)
		config.SetEnvPrefix(d.NS)
	}
	config.AutomaticEnv()
	d.app = app
	d.SequenceBase = config.GetUint64("sequence_base")
	d.SequenceKey = config.GetString("sequence_key")
	return nil
}

// Object implements Extension interface
func (d *SequenceGeneratorExt) Object() interface{} {
	return d
}

// Application implements Extension interface
func (d *SequenceGeneratorExt) Application() *gobay.Application {
	return d.app
}

// Close implements Extension interface
func (d *SequenceGeneratorExt) Close() error {
	return nil
}

// 当 step > 1 时, 即分配了一批 sequence, 可以使用 (sequence - step, sequence] 间的 sequence,
// 注意此时在分布式环境下 sequence 并不能保证随着时间递增
func (g *SequenceGeneratorExt) getSequence(step uint64) (uint64, error) {
	if step < 1 || step > maxSequence {
		return 0, fmt.Errorf("step should not less than 1 or greater than MAX_STEP(%d)", maxStep)
	}
	if g.redisClient == nil {
		g.redisClient = g.app.Get(g.RedisExtName).Object().(*redis.Client)
	}
	cmd := g.redisClient.Eval(luaScript, []string{g.SequenceKey}, maxSequence, step)
	result, err := cmd.Result()
	if err != nil {
		return 0, err
	}
	values := result.([]interface{})
	increment := uint64(values[0].(int64))
	currentSeconds := uint64(values[1].(int64))
	currentMicroseconds := uint64(values[2].(int64))
	shiftedMillisecond := ((currentSeconds-beginningTimestamp)*1000 + currentMicroseconds/1000) << timestampShift
	sequence := shiftedMillisecond + increment + g.SequenceBase
	return sequence, nil
}

func (g *SequenceGeneratorExt) GetSequence() (uint64, error) {
	return g.getSequence(1)
}

// 批量生成 sequence, 减少 redis 请求
// count: 需要生成的 sequence 数量
// batch_size: 单词请求 redis 取得的 sequence 数量,
// 调用 redis 生成 sequence 的 QPS 不能超过 (MAX_SEQUENCE / batch_size + 1) * 1000 => batch_size = MAX_SEQUENCE / (MAX_QPS / 1000 - 1)
// 以单台 redis 极限 4w QPS 计算(单条命令 25us), batch_size = 210.0512820513
func (g *SequenceGeneratorExt) GetSequences(count, batchSize uint64) *Sequences {
	return &Sequences{
		g:         g,
		restCount: count,
		batchSize: batchSize,
	}
}

type Sequences struct {
	g                    *SequenceGeneratorExt
	restCount, batchSize uint64

	mux             sync.Mutex
	lastMaxSequence uint64
	lastSequence    uint64

	// 用于减少内存分配次数
	step uint64
	err  error
}

func (s *Sequences) HasNext() bool {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.restCount > 0
}

func (s *Sequences) Next() (uint64, error) {
	s.mux.Lock()
	defer s.mux.Unlock()

	if s.restCount == 0 {
		return s.lastSequence, nil
	}
	if s.lastSequence < s.lastMaxSequence {
		s.lastSequence += 1
		s.restCount -= 1
		return s.lastSequence, nil
	}
	if s.restCount < s.batchSize {
		s.step = s.restCount
	} else {
		s.step = s.batchSize
	}

	s.lastMaxSequence, s.err = s.g.getSequence(s.step)
	if s.err != nil {
		s.restCount = 0
		return 0, s.err
	}
	s.lastSequence = s.lastMaxSequence - s.step + 1
	s.restCount -= 1
	return s.lastSequence, nil
}
