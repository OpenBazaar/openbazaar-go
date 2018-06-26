install:
		cd cmd/spvwallet && go install

protos:
		cd api/pb && protoc --go_out=plugins=grpc:. api.proto

resources:
		cd gui && go-bindata -o resources.go -pkg gui resources/...
