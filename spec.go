package configuration

import (
	"github.com/the-anna-project/context"
)

// Service represents a management layer providing configuration primitives for
// the internals of the neural network and its several sub-systems. Different
// configuration scenarios can be covered using the configuration service. This
// goes from simple, single label configurations, to multiple, nested
// configuration sets. This also works either with or without runtime variables
// being attached to configuration keys.
//
// There are variable amounts of labels being used to associate configuration
// profiles with them.
//
//     label
//     label
//     label
//
// There are variable amounts of namespaces being created in a reproducible way
// to organize the internal data structures of configuration profiles.
//
//     namespace
//     namespace
//     namespace
//
// There are several rulers. Registered rulers provide algorithms to decide
// which piece of configuration should be choosen. Rulers them self are
// represented as states internally.
//
//     ruler
//     ruler
//     ruler
//
// There are huge amounts of pieces. Created pieces represent a specific aspect
// of some configuration. Rulers them self are represented as states internally.
//
//     piece
//     piece
//     piece
//
// There are two different states. Right and wrong. These states represent
// statistics about binary decisions rulers made for pieces. Note that rulers
// and pieces are statistically recorded by states each. This is done to
// evaluate the success and error rates to dynamically learn about decisions
// being made over the lifetime of the neural network.
//
//     state
//     state
//
//     right
//     wrong
//
// The following storage key structures might be used.
//
//    service:configuration:kind:ruler:namespace:$namespace:state:right    $count
//    service:configuration:kind:ruler:namespace:$namespace:state:wrong    $count
//
//    service:configuration:kind:piece:namespace:$namespace:state:right    $count
//    service:configuration:kind:piece:namespace:$namespace:state:wrong    $count
//
type Service interface {
	// Create provides a way to add a key and its eventual result variables to a
	// specific set of configuration. Create can be called multiple times.
	// Consecutive calls with the same key overwrite its associated values with
	// the given ones.
	Create(ctx context.Context, labels []string, key string, results []interface{}) error
	// Delete removes the set of configuration and all its associated statistics
	// associated with the given labels.
	Delete(ctx context.Context, labels []string) error
	// Execute implements the success stage of the service. Here trial and replay
	// routines in context to the service's business logic are implemented.
	//
	// It returns one key and its associated results as being added using
	// Service.Create beforehand. There might be random, or even unknown, dynamic
	// algorithms used to select some key/results pair depending on the advances
	// of the neural network. Calling Service.Execute without having
	// Service.Create called beforehand with the according labels throws an error.
	// In case Service.Create has been called without any result variables,
	// Service.Execute returns an empty interface list as second return value.
	// That is, Service.Execute will not return an error in case there are no
	// result variables.
	Execute(ctx context.Context, labels []string) (string, []interface{}, error)
	// Failure implements the success stage of the service. Here statistical
	// records can be tracked. Further all state, but statistical records generate
	// during the execute stage, will be prurged.
	Failure(ctx context.Context, labels []string) error
	// Success implements the success stage of the service. Here statistical
	// records can be tracked.
	Success(ctx context.Context, labels []string) error
}
