package tui

import (
    "reflect"
    "sync"
    "testing"
)

func TestStoreHasRWMutex(t *testing.T) {
    storeType := reflect.TypeOf(Store{})
    field, ok := storeType.FieldByName("mu")
    if !ok {
        t.Fatal("Store is missing an RWMutex field named mu")
    }
    if field.Type != reflect.TypeOf(sync.RWMutex{}) {
        t.Fatalf("Store mu field is %v, want sync.RWMutex", field.Type)
    }
}
