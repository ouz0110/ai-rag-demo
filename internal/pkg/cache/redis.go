package cache

import (
	"context"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

type Serializer[T any] interface {
	Unmarshal(s string) (T, error)
	Marshal(t T) (string, error)
}

type NoOpsCacheSerializer[T ~string] struct{}

func (d NoOpsCacheSerializer[T]) Unmarshal(s string) (T, error) { return T(s), nil }
func (d NoOpsCacheSerializer[T]) Marshal(t T) (string, error)   { return string(t), nil }

type SerializerFunc[T any] struct {
	UnmarshalFunc func(s string) (T, error)
	MarshalFunc   func(t T) (string, error)
}

func (d SerializerFunc[T]) Unmarshal(s string) (T, error) { return d.UnmarshalFunc(s) }
func (d SerializerFunc[T]) Marshal(t T) (string, error)   { return d.MarshalFunc(t) }

type Helper[T any] struct {
	cli        *redis.Client
	expiration time.Duration
	serializer Serializer[T]
	formatKey  func(string) string
}

func NewHelper[T any](
	cli *redis.Client,
	expiration time.Duration,
	serializer Serializer[T],
	formatKey func(string) string,
) *Helper[T] {
	return &Helper[T]{
		cli:        cli,
		expiration: expiration,
		serializer: serializer,
		formatKey:  formatKey,
	}
}

func (c *Helper[T]) Set(ctx context.Context, key string, t T) error {
	str, err := c.serializer.Marshal(t)
	if err != nil {
		return err
	}
	_, err = c.cli.Set(ctx, c.formatKey(key), str, c.expiration).Result()
	if err != nil {
		return err
	}

	return nil
}

func (c *Helper[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var t T
	r, err := c.cli.Get(ctx, c.formatKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return t, false, nil
		}

		return t, false, err
	}
	t, err = c.serializer.Unmarshal(r)
	if err != nil {
		return t, false, err
	}

	return t, true, nil
}

func (c *Helper[T]) GetDel(ctx context.Context, key string) (T, bool, error) {
	var t T
	r, err := c.cli.GetDel(ctx, c.formatKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return t, false, nil
		}

		return t, false, err
	}
	t, err = c.serializer.Unmarshal(r)
	if err != nil {
		return t, false, err
	}

	return t, true, nil
}

func (c *Helper[T]) Del(ctx context.Context, keys ...string) error {
	keys = pie.Map(keys, func(key string) string { return c.formatKey(key) })
	_, err := c.cli.Del(ctx, keys...).Result()
	return err
}

func (c *Helper[T]) LLen(ctx context.Context, key string) (int, error) {
	l, err := c.cli.LLen(ctx, c.formatKey(key)).Result()
	return int(l), err
}

func (c *Helper[T]) LPush(ctx context.Context, key string, ts ...T) error {
	if len(ts) == 0 {
		return nil
	}

	vs := make([]interface{}, 0, len(ts))
	for _, t := range ts {
		s, err := c.serializer.Marshal(t)
		if err != nil {
			return err
		}
		vs = append(vs, s)
	}
	_, err := c.cli.LPush(ctx, c.formatKey(key), vs...).Result()
	return err
}

func (c *Helper[T]) LRange(ctx context.Context, key string, start, stop int) ([]T, error) {
	list, err := c.cli.LRange(ctx, c.formatKey(key), int64(start), int64(stop)).Result()
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return []T{}, nil
	}

	res := make([]T, 0, len(list))
	for _, v := range list {
		t, err := c.serializer.Unmarshal(v)
		if err != nil {
			return nil, err
		}
		res = append(res, t)
	}

	return res, nil
}
