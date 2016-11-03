# jsonpb
Modified Protobuf to JSON serializer in Go

Forked from the protobuf library to fix a couple issues:

1) Go type uint64 was always serialized as a string regarless if the number was large or not. This changes it to only serialize numbers as strings if they are greater than 2^32 (regardless of Go type).

2) When `EmitDefaults` is set to false, it should not fail to emit enums with a value of zero.
