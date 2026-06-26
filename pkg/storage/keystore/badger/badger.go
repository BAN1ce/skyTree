package badger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dgraph-io/badger"
)

type Badger struct {
	db *badger.DB
}

func NewBadger(options badger.Options) (*Badger, error) {
	db, err := badger.Open(options)
	if err != nil {
		return nil, err
	}
	return &Badger{db: db}, nil
}

func (b *Badger) HSet(ctx context.Context, key []byte, field [][]byte) error {
	var errs error
	if len(field)%2 != 0 {
		return fmt.Errorf("invalid HSet field: expected even number of entries, got %d", len(field))
	}

	for i := 0; i < len(field); i += 2 {
		value := field[i+1]
		err := b.PutKey(ctx, append(key, field[i]...), value)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("put %s, %w", field[i], err))
		}
	}
	return errs
}

func (b *Badger) HGet(ctx context.Context, key, field []byte) ([]byte, bool, error) {
	return b.ReadKey(ctx, append(key, field...))
}

func (b *Badger) HDel(ctx context.Context, key []byte, field [][]byte) error {
	for _, f := range field {
		err := b.DeleteKey(ctx, append(key, f...))
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Badger) HGetAll(ctx context.Context, key []byte) (map[string]string, error) {
	result, err := b.ReadPrefixKey(ctx, key)
	return result, err
}

func (b *Badger) HPrefix(ctx context.Context, key []byte, prefix []byte) (map[string]string, error) {
	return b.ReadPrefixKey(ctx, append(key, prefix...))
}

func (b *Badger) DeleteHash(ctx context.Context, key []byte) error {
	return b.DeletePrefixKey(ctx, key)
}

func (b *Badger) PutKey(ctx context.Context, key, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (b *Badger) ReadKey(ctx context.Context, key []byte) (value []byte, ok bool, err error) {
	err = b.db.View(func(txn *badger.Txn) error {
		item, err1 := txn.Get(key)
		if errors.Is(err1, badger.ErrKeyNotFound) {
			return nil
		}
		if err1 != nil {
			return err1
		}

		return item.Value(func(val []byte) error {
			value = append([]byte(nil), val...)
			ok = true
			return nil
		})
	})
	return
}

func (b *Badger) DeleteKey(ctx context.Context, key []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (b *Badger) ReadPrefixKey(ctx context.Context, prefix []byte) (result map[string]string, err error) {
	result = make(map[string]string)

	err = b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()
			err := item.Value(func(v []byte) error {
				result[string(k)] = string(v)
				return nil
			})

			if err != nil {
				return err
			}
		}
		return nil

	})

	return result, err
}

func (b *Badger) DeletePrefixKey(ctx context.Context, prefix []byte) error {
	keys, err := b.ReadPrefixKey(ctx, prefix)
	if err != nil {
		return err
	}

	return b.db.Update(func(txn *badger.Txn) error {
		for k := range keys {
			err := txn.Delete([]byte(k))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *Badger) SetExpired(ctx context.Context, key []byte, duration time.Duration) error {
	if duration <= 0 {
		return fmt.Errorf("set expired: duration must be > 0, got %s", duration)
	}
	return b.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return txn.SetEntry(badger.NewEntry(key, value).WithTTL(duration))
	})
}

func (b *Badger) Close() error {
	return b.db.Close()
}

func (b *Badger) Snapshot(writer io.Writer) error {
	_, err := b.db.Backup(writer, 0)
	return err
}

func (b *Badger) Recover(reader io.Reader) error {
	return b.db.Load(reader, 0)
}
