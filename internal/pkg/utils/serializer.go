package utils

import (
	"encoding/base64"
	"errors"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/go-kratos/kratos/v2/encoding"
	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

type JSONSerializer[T any] struct{}

func (p JSONSerializer[T]) Unmarshal(s string) (T, error) { return JSONUnmarshal[T](s) }
func (p JSONSerializer[T]) Marshal(t T) (string, error)   { return JSONMarshal(t) }

func JSONUnmarshal[T any](s string) (T, error) {
	var t T // T 为值或指针都可以正确 Unmarshal。
	codec := encoding.GetCodec("json")
	err := codec.Unmarshal([]byte(s), &t)
	if err != nil {
		return t, err
	}

	return t, nil
}

func JSONMarshal(t any) (string, error) {
	codec := encoding.GetCodec("json")
	bytes, err := codec.Marshal(t)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
func ConvertToProtoStruct(a any) (*structpb.Struct, error) {
	jsonStr, err := JSONMarshal(a)
	if err != nil {
		return nil, err
	}
	return JSONUnmarshal[*structpb.Struct](jsonStr)
}

func ProtoEnumByName[T ~int32](s string) T {
	var v T
	t := interface{}(v).(protoreflect.Enum)
	bn := t.Descriptor().Values().ByName(protoreflect.Name(s))
	if bn == nil {
		return v
	}
	return T(bn.Number())
}

func JSONMarshalToSnakeCase(t any) (string, error) {
	m := structs.Map(t)
	for key, value := range m {
		underscoreKey := strcase.ToSnake(key)
		delete(m, key)
		m[underscoreKey] = value
	}

	return JSONMarshal(m)
}

func IgnoreErrorJSONMarshal(t any) string {
	bytes, err := JSONMarshal(t)
	if err != nil {
		return err.Error()
	}

	return bytes
}

func EncodeProtoAsBase64[T proto.Message](t T) (string, error) {
	bytes, err := proto.Marshal(t)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func DecodeBase64AsProto[T proto.Message](str string) (t T, err error) {
	var bs []byte
	// 早期数据中的 '=' 在 url 中可能被转译为 %3D
	str = strings.ReplaceAll(str, "%25", "%")
	str = strings.ReplaceAll(str, "%3D", "=")
	bs, err = base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		// 兼容早期已经被持久化的数据
		bs, err = base64.URLEncoding.DecodeString(str)
		if err != nil {
			bs, err = base64.StdEncoding.DecodeString(str)
			if err != nil {
				return
			}
		}
	}
	pm, err := newProtoMessage(t)
	if err != nil {
		return
	}
	t = pm.(T)
	err = proto.Unmarshal(bs, t)
	return
}

func newProtoMessage(v interface{}) (proto.Message, error) {
	if _, ok := v.(proto.Message); !ok {
		return nil, errors.New("not proto message")
	}

	nv := reflect.New(reflect.TypeOf(v).Elem())
	return nv.Interface().(proto.Message), nil
}
