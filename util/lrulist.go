package util

import (
	"container/list"
	"sync"
)

type Keyer[K comparable] interface {
	GetKey() K
}

type LRUList[K comparable, V Keyer[K]] struct {
	mu    sync.Mutex
	ll    *list.List
	items map[K]*list.Element
}

func NewLRUList[K comparable, V Keyer[K]]() *LRUList[K, V] {
	return &LRUList[K, V]{
		ll:    list.New(),
		items: make(map[K]*list.Element),
	}
}

func (l *LRUList[K, V]) AddOrTouch(val V) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := val.GetKey()

	if elem, ok := l.items[key]; ok {
		elem.Value = val
		l.ll.MoveToFront(elem)
		return
	}

	elem := l.ll.PushFront(val)
	l.items[key] = elem
}

func (l *LRUList[K, V]) GetAt(index int) (V, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var zero V

	if index < 0 || index >= l.ll.Len() {
		return zero, nil
	}

	elem := l.ll.Front()
	for i := 0; i < index; i++ {
		elem = elem.Next()
	}

	l.ll.MoveToFront(elem)
	return elem.Value.(V), nil
}

func (l *LRUList[K, V]) GetAll() []V {
	l.mu.Lock()
	defer l.mu.Unlock()

	items := make([]V, 0, l.ll.Len())
	for elem := l.ll.Front(); elem != nil; elem = elem.Next() {
		items = append(items, elem.Value.(V))
	}
	return items
}

func (l *LRUList[K, V]) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ll.Len()
}

func (l *LRUList[K, V]) Remove(key K) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if elem, ok := l.items[key]; ok {
		l.ll.Remove(elem)
		delete(l.items, key)
	}
}
