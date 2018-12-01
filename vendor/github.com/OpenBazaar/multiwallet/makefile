install:
		cd cmd/multiwallet && go install

protos:
		cd api/pb && protoc --go_out=plugins=grpc:. api.proto