package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	BadRequestError = status.Error(codes.InvalidArgument, "Invalid Argument")
)
