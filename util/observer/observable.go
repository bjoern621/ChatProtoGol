package observer

import (
	"sync"

	"bjoernblessin.de/chatprotogol/util/logger"
)

// Observable manages a set of subscribers (channels) that receive notifications.
type Observable[T any] struct {
	observers  map[chan T]struct{}
	mu         sync.RWMutex
	closed     bool
	bufferSize int
}

// NewObservable creates a new Observable instance.
// bufferSize specifies the size of the channel buffer for each subscriber (0: unbuffered, 1+: size of buffer).
// New messages will be discarded if the subscriber's channel is full.
// Example: stringObservable := NewObservable[string]() creates an observable for string events.
func NewObservable[T any](bufferSize int) *Observable[T] {
	return &Observable[T]{
		observers:  make(map[chan T]struct{}),
		bufferSize: bufferSize,
	}
}

// Subscribe adds a new subscriber and returns a channel for receiving notifications.
// The caller is responsible for consuming from the returned channel.
// The channel will be closed when Unsubscribe is called or when the Observable is closed.
// Example: msgChannel := myObservable.Subscribe() will return a new channel msgChannel that will receive notifications of type T.
func (o *Observable[T]) Subscribe() chan T {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		ch := make(chan T)
		close(ch)
		return ch
	}

	ch := make(chan T, o.bufferSize)
	o.observers[ch] = struct{}{}
	return ch
}

// SubscribeOnce adds a subscriber that will receive only one notification.
// After the first notification is sent to the returned channel, the subscription is automatically removed and the channel is closed.
// Example: oneTimeChannel := myObservable.SubscribeOnce() will return a channel that receives one notification and then closes.
func (o *Observable[T]) SubscribeOnce() chan T {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		ch := make(chan T)
		close(ch)
		return ch
	}

	ch := make(chan T, 1)
	o.observers[ch] = struct{}{}

	go func() {
		// Wait for one message or for the channel to be closed by Unsubscribe/Close
		_, ok := <-ch
		if ok {
			o.Unsubscribe(ch) // Unsubscribe after receiving the message, if it was not closed already
		}
	}()

	return ch
}

// Unsubscribe removes a subscriber channel.
// It closes the provided channel to signal the subscriber that no more notifications will be sent.
// Example: myObservable.Unsubscribe(msgChannel) will remove msgChannel from the subscribers and close it.
func (o *Observable[T]) Unsubscribe(ch chan T) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, ok := o.observers[ch]; ok {
		delete(o.observers, ch)
		close(ch)
	}
}

// NotifyObservers sends data to all currently subscribed channels.
// This operation is non-blocking for the Observable. If a subscriber's channel buffer is full,
// the notification for that subscriber might be dropped to prevent blocking NotifyObservers.
// Example: myObservable.NotifyObservers("hello world") will send "hello world" to all subscribed channels.
func (o *Observable[T]) NotifyObservers(data T) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed {
		return
	}

	for ch := range o.observers {
		select {
		case ch <- data:
		default:
			// Subscriber channel is full or closed, skip sending to this one
			logger.Warnf("Observable[%T](%p): Subscriber channel is full or closed, skipping notification", data, o)
		}
	}
}

// NotifyObservers is similar to NotifyObservers but blocks until all subscribers have received the data.
func (o *Observable[T]) NotifyObserversBlock(data T) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed {
		return
	}

	for ch := range o.observers {
		ch <- data
	}
}

// ClearAllSubscribers removes all subscribers and closes their respective channels.
// Example: myObservable.ClearAllSubscribers() will remove and close all subscriber channels.
func (o *Observable[T]) ClearAllSubscribers() {
	o.mu.Lock()
	defer o.mu.Unlock()

	for ch := range o.observers {
		delete(o.observers, ch)
		close(ch)
	}
}

// Close closes the observable, unsubscribes all current subscribers, and prevents new subscriptions.
// Example: myObservable.Close() will shut down the observable.
func (o *Observable[T]) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return
	}

	o.closed = true
	for ch := range o.observers {
		delete(o.observers, ch)
		close(ch)
	}
}
