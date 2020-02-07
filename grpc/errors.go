package gobay_grpc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	NotFoundError      = status.Error(codes.NotFound, "Not Found")
	AlreadyExistsError = status.Error(codes.AlreadyExists, "Already Exists")
	BadRequestError    = status.Error(codes.InvalidArgument, "Invalid Argument")
)
