# Quick Start

```sh
gobay new <your-project>

# i.e.
gobay new github.com/company/helloworld-project
cd helloworld-project
```

- Run the tools in docker dev box if you like:

```sh
cd dockerfiles
sh run.sh
```

---

## To start a grpc server

```sh
make run COMMAND=grpcsvc
```

then `grpc.health.v1.health/check` is available on port 6000.

---

## To start an API server

1. We need to generate some openapi code with the openapi spec (`spec/openapi/main.yml`) (require openapi tool, use docker dev box if needed)

```sh
make genswagger
```

1. Start the server

```sh
make run COMMAND=httpsvc
```

Then you may view the api docs at [http://127.0.0.1:5000/helloworld-project/docs](http://127.0.0.1:5000/helloworld-project/docs)
