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

func GetByID[T any](ctx *Context, bucketID BucketID, ID ID128, result *T) bool {
	var found bool
	ctx.Database.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketID))
		if bucket == nil {
			return BucketNotFoundError{}
		}

		_result := bucket.Get(ID[:])
		if _result == nil {
			return nil
		}

		err := Unpack(_result, result)
		if err == nil {
			found = true
		} else {
			log.Println("Unpack FAILED: ", err)
		}

		return nil
	})

	return found
}

func InsertByID[T any](ctx *Context, bucketID BucketID, ID ID128, value *T) error {
	err := ctx.Database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketID))
		if bucket == nil {
			return BucketNotFoundError{}
		}

		_result, err := Pack(value)
		if err != nil {
			return err
		}

		return bucket.Put(ID[:], _result)
	})

	return err
}

func Insert[T any](bucket *bolt.Bucket, ID ID128, value *T) error {
	binaryData, err := Pack(value)
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

func IterateCollect[V any](bucket *bolt.Bucket, iteratorProcedure func(key ID128, value *V) bool) []V {
	cursor := bucket.Cursor()
	var result []V
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		var theStruct V
		err := Unpack(value, &theStruct)
		if err != nil {
			log.Printf("Unpack FAILED for ID %v, reason: %v", key, err)
		}

		if !iteratorProcedure(ID128(key), &theStruct) {
			break
		}

		result = append(result, theStruct)
	}

	return result
}

func IterateCollectAll[V any](bucket *bolt.Bucket) []V {
	cursor := bucket.Cursor()
	var result []V
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		var theStruct V
		err := Unpack(value, &theStruct)
		if err != nil {
			log.Printf("Unpack FAILED for ID %v, reason: %v", key, err)
		}

		result = append(result, theStruct)
	}

	return result
}

func IterateRemove[V any](bucket *bolt.Bucket, iteratorProcedure func(key ID128, value *V) bool) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		var theStruct V
		err := Unpack(value, &theStruct)
		if err != nil {
			log.Printf("Unpack FAILED for ID %v, reason: %v", key, err)
		}

		if iteratorProcedure(ID128(key), &theStruct) {
			bucket.Delete(key)
		}
	}
}

func IterateFind[V any](bucket *bolt.Bucket, target *V, iteratorProcedure func(key ID128, value *V) bool) (found bool) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		var theStruct V
		err := Unpack(value, &theStruct)
		if err != nil {
			log.Printf("Unpack FAILED for ID %v, reason: %v", key, err)
		}

		if iteratorProcedure(ID128(key), &theStruct) {
			*target = theStruct
			found = true
			break
		}
	}

	return
}

func WriteTx(ctx *Context) (*bolt.Tx, error) {
	return ctx.Database.Begin(true)
}

func ReadTx(ctx *Context) (*bolt.Tx, error) {
	return ctx.Database.Begin(false)
}
