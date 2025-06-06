package observer

type Observer[T any] interface {
	// Update is called when the observable notifies its observers.
	Update(data T)
}
