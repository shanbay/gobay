#!/bin/bash

docker build -t gobay-dev:latest -f Dockerfile-dev .

docker run -it -v $(pwd)/..:/go/src/app gobay-dev:latest
