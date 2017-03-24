package join

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoin(t *testing.T) {
	score := func(left, right interface{}) int {
		return left.(int) - right.(int)
	}

	pairs, left, right := Join([]int{10, 11, 12}, []int{10, 11, 12}, score)
	assert.Zero(t, len(left))
	assert.Zero(t, len(right))
	assert.Equal(t, []Pair{{10, 10}, {11, 11}, {12, 12}}, pairs)

	pairs, left, right = Join([]int{10, 11, 12}, []int{13, 1, 2}, score)
	assert.Equal(t, []interface{}{12}, left)
	assert.Equal(t, []interface{}{13}, right)
	assert.Equal(t, []Pair{{10, 2}, {11, 1}}, pairs)
}

type JoinList []interface{}

func (jil JoinList) Len() int {
	return len(jil)
}

func (jil JoinList) Get(ii int) interface{} {
	return jil[ii]
}

type JoinInt int

func (ji JoinInt) JoinKey() interface{} {
	return ji
}

func TestHashJoin(t *testing.T) {
	keyFunc := func(val interface{}) interface{} {
		return val
	}
	pairs, left, right := HashJoin(JoinList{10, 11, 12},
		JoinList{10, 11, 12}, keyFunc, keyFunc)
	assert.Len(t, left, 0)
	assert.Len(t, right, 0)
	assert.Equal(t, []Pair{{10, 10}, {11, 11}, {12, 12}}, pairs)

	pairs, left, right = HashJoin(JoinList{10, 11, 12},
		JoinList{13, 11, 2}, keyFunc, keyFunc)
	assert.Len(t, left, 2)
	assert.Len(t, right, 2)
	assert.Equal(t, []Pair{{11, 11}}, pairs)
}

func TestHashJoinNilKeyFunc(t *testing.T) {
	keyFunc := func(val interface{}) interface{} {
		return val
	}
	pairs, left, right := HashJoin(JoinList{10, 11, 12},
		JoinList{10, 11, 12}, nil, keyFunc)
	assert.Len(t, left, 0)
	assert.Len(t, right, 0)
	assert.Equal(t, []Pair{{10, 10}, {11, 11}, {12, 12}}, pairs)

	pairs, left, right = HashJoin(JoinList{10, 11, 12},
		JoinList{13, 11, 2}, keyFunc, nil)
	assert.Len(t, left, 2)
	assert.Len(t, right, 2)
	assert.Equal(t, []Pair{{11, 11}}, pairs)
}

func TestHashJoinUnHashableKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("HashJoin did not panic on unhashable key")
		}
	}()

	keyFunc := func(val interface{}) interface{} {
		return make([]int, 16)
	}
	HashJoin(JoinList{10, 11, 12}, JoinList{10, 11, 12}, keyFunc, keyFunc)
}

func ExampleJoin() {
	lefts := []string{"a", "bc", "def"}
	rights := []int{0, 2, 4}
	score := func(left, right interface{}) int {
		return len(left.(string)) - right.(int)
	}
	pairs, lonelyLefts, lonelyRights := Join(lefts, rights, score)

	fmt.Println(pairs, lonelyLefts, lonelyRights)
	// Output: [{a 0} {bc 2}] [def] [4]
}
