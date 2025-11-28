// Package invoke предоставляет реализации сериализаторов для модуля Invoke.
package invoke

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"potter/framework/transport"
)

// JSONSerializer реализация JSON сериализатора
type JSONSerializer struct{}

// NewJSONSerializer создает новый JSON сериализатор
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize сериализует сообщение в JSON
func (s *JSONSerializer) Serialize(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}

// Deserialize десериализует JSON в сообщение
func (s *JSONSerializer) Deserialize(data []byte, msg interface{}) error {
	return json.Unmarshal(data, msg)
}

// ProtobufSerializer реализация Protobuf сериализатора
type ProtobufSerializer struct{}

// NewProtobufSerializer создает новый Protobuf сериализатор
func NewProtobufSerializer() *ProtobufSerializer {
	return &ProtobufSerializer{}
}

// Serialize сериализует сообщение в Protobuf
func (s *ProtobufSerializer) Serialize(msg interface{}) ([]byte, error) {
	pb, ok := msg.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("message does not implement proto.Message: %T", msg)
	}
	return proto.Marshal(pb)
}

// Deserialize десериализует Protobuf в сообщение
func (s *ProtobufSerializer) Deserialize(data []byte, msg interface{}) error {
	pb, ok := msg.(proto.Message)
	if !ok {
		return fmt.Errorf("message does not implement proto.Message: %T", msg)
	}
	return proto.Unmarshal(data, pb)
}

// MessagePackSerializer реализация MessagePack сериализатора
// Требует зависимость: github.com/vmihailenco/msgpack/v5
// Для использования добавьте в go.mod: require github.com/vmihailenco/msgpack/v5 v5.4.0
type MessagePackSerializer struct{}

// NewMessagePackSerializer создает новый MessagePack сериализатор
// ВАЖНО: Требует установки зависимости github.com/vmihailenco/msgpack/v5
func NewMessagePackSerializer() *MessagePackSerializer {
	return &MessagePackSerializer{}
}

// Serialize сериализует сообщение в MessagePack
// ВАЖНО: Требует установки зависимости github.com/vmihailenco/msgpack/v5
func (s *MessagePackSerializer) Serialize(msg interface{}) ([]byte, error) {
	// Раскомментируйте после установки зависимости:
	// return msgpack.Marshal(msg)
	return nil, fmt.Errorf("msgpack serializer requires github.com/vmihailenco/msgpack/v5 dependency")
}

// Deserialize десериализует MessagePack в сообщение
// ВАЖНО: Требует установки зависимости github.com/vmihailenco/msgpack/v5
func (s *MessagePackSerializer) Deserialize(data []byte, msg interface{}) error {
	// Раскомментируйте после установки зависимости:
	// return msgpack.Unmarshal(data, msg)
	return fmt.Errorf("msgpack serializer requires github.com/vmihailenco/msgpack/v5 dependency")
}

// DefaultSerializer возвращает сериализатор по умолчанию (JSON)
func DefaultSerializer() transport.MessageSerializer {
	return NewJSONSerializer()
}

