package easyframework

import (
	"github.com/boltdb/bolt"
	"log"
)

type BucketID string

func NewBucket(ctx *Context, bucketID BucketID) error {
	tx, err := ctx.Database.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingBucket := tx.Bucket([]byte(bucketID))
	if existingBucket == nil {
		_, err := tx.CreateBucket([]byte(bucketID))
		if err != nil {
			return err
		}
		log.Printf("Creating bucket: %v", bucketID)
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

type BucketNotFoundError struct{}

func (v BucketNotFoundError) Error() string {
	return "Bucket not found"
}

func GetBucket(tx *bolt.Tx, id BucketID) (*bolt.Bucket, error) {
	bucket := tx.Bucket([]byte(id))
	if bucket == nil {
		return nil, BucketNotFoundError{}
	}

	return bucket, nil
}

func GetByID[T any](ctx Context, bucketID BucketID, ID ID128) (*T, error) {
	var result *T
	err := ctx.Database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketID))
		if bucket == nil {
			return BucketNotFoundError{}
		}

		_result := bucket.Get([]byte(ID[:]))
		if _result == nil {
			return nil
		}

		result = new(T)
		return Unpack(_result, result)
	})

	return result, err
}

func InsertByID[T any](ctx Context, bucketID BucketID, ID ID128, value T) error {
	err := ctx.Database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketID))
		if bucket == nil {
			return BucketNotFoundError{}
		}

		_result, err := Pack(&value)
		if err != nil {
			return err
		}

		return bucket.Put(ID[:], _result)
	})

	return err
}

func Insert[T any](bucket *bolt.Bucket, ID ID128, value T) error {
	binaryData, err := Pack(&value)
	if err != nil {
		return err
	}
	err = bucket.Put(ID[:], binaryData)
	if err != nil {
		return err
	}

	return nil
}

func Iterate[V any](bucket *bolt.Bucket, iteratorProcedure func(key ID128, value *V) bool) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		var theStruct V
		err := Unpack(value, &theStruct)
		if err != nil {
			log.Printf("Unpack FAILED for ID %v, reason: %v", key, err)
		}

		if !iteratorProcedure(ID128(key), &theStruct) {
			break
		}
	}
}

func WriteTx(ctx *Context) (*bolt.Tx, error) {
	return ctx.Database.Begin(true)
}

func ReadTx(ctx *Context) (*bolt.Tx, error) {
	return ctx.Database.Begin(false)
}
