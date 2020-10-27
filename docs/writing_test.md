# Writing tests

```go
package testedpackagename

import (
    ...

    mock_something "git.17bdc.com/shanbay/protos-go/mocks/something"

    "github.com/shanbay/gobay/utils/testhelpers"
)

func setup() *gobay.Application {
    // init app
    curdir, _ := os.Getwd()
    root := path.Join(curdir, "..", "..")
    extensions := app.Extensions()
    bapp, err := gobay.CreateApp(root, "testing", extensions)
    if err != nil {
        panic(err)
    }
    app.InitExts(bapp)

    // prepare cache
    models.InitCaches()

    // migrate db
    app.EntClient.Schema.Create(context.Background())
    return bapp
}

func tearDown() {
	  ctx := context.Background()

    // clear db tables
    app.EntClient.SomeDBModel.Delete().ExecX(ctx)

    // clear redis
    redisclient := app.Redis.Client(context.Background())
    redisclient.FlushDB()

    // clear cache
    app.Cache.Delete(ctx, "somecachekey")
}

// mock db data
func fixtureSomeDBModels() {
    // create data in db/cache/etc. for testing
}

// mock rpc method
func mockSomeRPC(t *testing.T) *gomock.Controller {
    ctrl := gomock.NewController(t)
    mockedClient := mock_something.NewMockSomeClient(ctrl)
    app.SomeClient = mockedClient

    mockedClient.EXPECT().SomeRpcMethod(
      gomock.Any(), gomock.Any(), gomock.Any(),
    ).Return(
      &something.RespType{}, nil,
    ).AnyTimes()
}

func tearDownRPCMock(ctrl *gomock.Controller) {
	  ctrl.Finish()
}

func Test_newAdminGetUserAppletAndBuyRecordHandler(t *testing.T) {
    bapp := setup()
    fixtureSomeDBModels()
    defer tearDown()

    ctx := context.Background()
    ctrl := mockSomeRPC(t)
    defer tearDownRPCMock(ctrl)

    handler := getHandler(bapp)

    // create test cases
    testCases := []testhelpers.TestCase{
      testhelpers.MakeTestCase(
        &testhelpers.TestCase{
          App:  bapp,
          Ctx:  ctx,
          Name: "TestSuccessName",
          Req: &gen_applet.SomeRequestParams{
            UserID:   101,
            AppletID: 1,
          },
          IgnoredFieldKeys: []string{"to_ignore_id.id"},
        },
        &openapi_models.Something{
          Key1: ,
          ToIgnoreId: &openapi_models.SomethingElse{
            Status:     "IN_USE",
            AutoResume: true,
            ID:         404,
          },
        }),
      {
        App:  bapp,
        Ctx:  ctx,
        Name: "TestErrorName",
        Req: &gen_applet.SomeRequestParams{
          UserID:   123,
          AppletID: 456,
        },
        WantErr:        true,
        WantStatusCode: 400,
      },
    }

    // API Tests ----------------

    // create getRequest Function for running test cases
    getAPIRequestFunc := func(req interface{}) *http.Request {
      reqBody := req.(*gen_applet.SomeRequestParams)
      result := getRequest("GET", "/something/"+reqBody.UserID+"/something/"+reqBody.AppletID, nil)
      return result
    }

    // check API test cases
    testhelpers.CheckAPITestCases(testCases, getAPIRequestFunc, t, handler)

    // OR

    // if want to manually check test result
    for _, tt := range tests {
        t.Run(tt.Name, func(t *testing.T) {
            req := getRequest(tt.Req)
            res := httptest.NewRecorder()
            handler.ServeHTTP(res, req)

            // check if tt.WantJSON == res.Body.(JSON)
            CheckAPITestCaseResult(tt, res, t)
        })
    }

    // GRPC Tests ----------------

    // create get GRPC result function for running test cases
    getGRPCResultFunc := func(test TestCase, t *testing.T) (interface{}, error) {
        s := &someRPCServer{
            app: test.App,
        }
        typedReq := test.Req.(*RpcRequestType)
        got, err := s.someRPRCFunc(test.Ctx, typedReq)
        return got, err
    }

    // check GRPC test cases
    testhelpers.CheckGRPCTestCases(testCases, getGRPCResultFunc, t)

    // OR

    // if want to manually check test result
    for _, tt := range testCases {
        t.Run(tt.Name, func(t *testing.T) {
            got, err := getGrpcResult(tt, t)

            CheckGRPCTestCaseResult(tt, got, err, t)
        })
    }
}
```
