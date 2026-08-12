package versioning

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

// activeLoader is set for the duration of Generate so legacy helpers use the shared cache.
var activeLoader *Loader

func findMessageDescriptor(file protoreflect.FileDescriptor, name string) protoreflect.MessageDescriptor {
	for i := 0; i < file.Messages().Len(); i++ {
		m := file.Messages().Get(i)
		if string(m.Name()) == name {
			return m
		}
	}
	return nil
}
