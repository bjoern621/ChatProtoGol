package observer

import "slices"

type Observable[T any] struct {
	observers []Observer[T]
}

// NewObservable creates a new Observable instance.
func NewObservable[T any]() *Observable[T] {
	return &Observable[T]{
		observers: make([]Observer[T], 0),
	}
}

// AddObserver adds an observer to the observable.
func (o *Observable[T]) AddObserver(observer Observer[T]) {
	o.observers = append(o.observers, observer)
}

// ObserveOnce adds an observer that will be notified only once.
// After the first notification, it will be removed automatically.
func (o *Observable[T]) ObserveOnce(observer Observer[T]) {
	wrapper := &onceObserver[T]{
		observable: o,
		observer:   observer,
	}
	o.observers = append(o.observers, wrapper)
}

// onceObserver is a wrapper that calls the original observer once and then removes itself
type onceObserver[T any] struct {
	observable *Observable[T]
	observer   Observer[T]
}

// Update calls the wrapped observer and then removes itself from the observable
func (o *onceObserver[T]) Update(data T) {
	o.observer.Update(data)
	o.observable.RemoveObserver(o)
}

// RemoveObserver removes an observer from the observable.
func (o *Observable[T]) RemoveObserver(observer Observer[T]) {
	for i, obs := range o.observers {
		if obs == observer {
			o.observers = slices.Delete(o.observers, i, i+1)
			return
		}
	}
}

// NotifyObservers notifies all observers with the given data.
func (o *Observable[T]) NotifyObservers(data T) {
	for _, observer := range o.observers {
		observer.Update(data)
	}
}

// ClearObservers removes all observers from the observable.
func (o *Observable[T]) ClearObservers() {
	o.observers = nil
}
