package configuration

import (
	"strings"
	"sync"

	"github.com/the-anna-project/context"
	currentstage "github.com/the-anna-project/context/current/stage"
	"github.com/the-anna-project/id"
	"github.com/the-anna-project/instrumentor"
	"github.com/the-anna-project/random"
	"github.com/the-anna-project/storage"
	"github.com/the-anna-project/worker"
)

const (
	randomRuler  = "random"
	highestRuler = "highest"
)

// ServiceConfig represents the configuration used to create a new service.
type ServiceConfig struct {
	// Dependencies.
	IDService              id.Service
	InstrumentorCollection *instrumentor.Collection
	RandomService          random.Service
	StorageCollection      *storage.Collection
	WorkerService          worker.Service
}

// DefaultServiceConfig provides a default configuration to create a new
// service by best effort.
func DefaultServiceConfig() ServiceConfig {
	var err error

	var idService id.Service
	{
		idConfig := id.DefaultServiceConfig()
		idService, err = id.NewService(idConfig)
		if err != nil {
			panic(err)
		}
	}

	var instrumentorCollection *instrumentor.Collection
	{
		instrumentorConfig := instrumentor.DefaultCollectionConfig()
		instrumentorCollection, err = instrumentor.NewCollection(instrumentorConfig)
		if err != nil {
			panic(err)
		}
	}

	var randomService random.Service
	{
		randomConfig := random.DefaultServiceConfig()
		randomService, err = random.NewService(randomConfig)
		if err != nil {
			panic(err)
		}
	}

	var storageCollection *storage.Collection
	{
		storageConfig := storage.DefaultCollectionConfig()
		storageCollection, err = storage.NewCollection(storageConfig)
		if err != nil {
			panic(err)
		}
	}

	var workerService worker.Service
	{
		workerConfig := worker.DefaultServiceConfig()
		workerService, err = worker.NewService(workerConfig)
		if err != nil {
			panic(err)
		}
	}

	config := ServiceConfig{
		// Dependencies.
		IDService:              idService,
		InstrumentorCollection: instrumentorCollection,
		RandomService:          randomService,
		StorageCollection:      storageCollection,
		WorkerService:          workerService,
	}

	return config
}

// NewService creates a new configured service.
func NewService(config ServiceConfig) (Service, error) {
	// Dependencies.
	if config.IDService == nil {
		return nil, maskAnyf(invalidConfigError, "id service must not be empty")
	}
	if config.InstrumentorCollection == nil {
		return nil, maskAnyf(invalidConfigError, "instrumentor collection must not be empty")
	}
	if config.RandomService == nil {
		return nil, maskAnyf(invalidConfigError, "random service must not be empty")
	}
	if config.StorageCollection == nil {
		return nil, maskAnyf(invalidConfigError, "storage collection must not be empty")
	}
	if config.WorkerService == nil {
		return nil, maskAnyf(invalidConfigError, "worker service must not be empty")
	}

	newService := &service{
		// Dependencies.
		id:           config.IDService,
		instrumentor: config.InstrumentorCollection,
		random:       config.RandomService,
		storage:      config.StorageCollection,
		worker:       config.WorkerService,

		// Internals.
		bootOnce:     sync.Once{},
		closer:       make(chan struct{}, 1),
		mutex:        sync.Mutex{},
		pieces:       map[string][]interface{}{},
		rulers:       map[string]func(ctx context.Context, labels []string) (string, error){},
		shutdownOnce: sync.Once{},
	}

	return newService, nil
}

type service struct {
	// Dependencies.
	id           id.Service
	instrumentor *instrumentor.Collection
	random       random.Service
	storage      *storage.Collection
	worker       worker.Service

	// Internals.
	bootOnce     sync.Once
	closer       chan struct{}
	mutex        sync.Mutex
	pieces       map[string][]interface{}
	rulers       map[string]func(ctx context.Context, labels []string) (string, error)
	shutdownOnce sync.Once
}

func (s *service) Boot() {
	s.bootOnce.Do(func() {
		// The following function implements the random ruler. It chooses the
		// namespace keys identified by pseudo random indizes.
		s.rulers[randomRuler] = func(ctx context.Context, labels []string) (string, error) {
			namespace := labelsToNamespace(labels...)
			key := pieceListKey(namespace)

			element, err := s.storage.Configuration.GetRandomFromScoredSet(key)
			if err != nil {
				return "", maskAny(err)
			}

			return element, nil
		}
		// The following function implements the highest ruler. It chooses the
		// namespace keys having the hightest right states.
		s.rulers[highestRuler] = func(ctx context.Context, labels []string) (string, error) {
			namespace := labelsToNamespace(labels...)
			key := pieceListKey(namespace)

			elements, err := s.storage.Configuration.GetHighestScoredElements(key, 1)
			if err != nil {
				return "", maskAny(err)
			}
			if len(elements) != 1 {
				// We actually want to fetch exactly one element that has the highest
				// score applied. In case there is no element returned, there might be
				// no element at all.
				return "", maskAny(notFoundError)
			}

			return elements[0], nil
		}
	})
}

func (s *service) Create(ctx context.Context, labels []string, pieceID string, results []interface{}) error {
	namespace := labelsToNamespace(labels...)

	// get current stage
	var currentStage currentstage.Value
	{
		var ok bool
		currentStage, ok = currentstage.FromContext(ctx)
		if !ok {
			return maskAnyf(invalidContextError, "current stage must not be empty")
		}
	}

	// Ensure rulers exists for the given namespace when this is the first trial.
	// Note that we have to check the existence since create can be called
	// multiple times.
	if currentStage.Trial() {
		key := rulerListKey(namespace)
		exists, err := s.storage.Configuration.Exists(key)
		if err != nil {
			return maskAny(err)
		}
		if !exists {
			for k, _ := range s.rulers {
				err := s.storage.Configuration.SetElementByScore(key, k, 0)
				if err != nil {
					return maskAny(err)
				}
			}
		}
	}

	//
	{
		key := pieceListKey(namespace)
		exists, err := s.storage.Configuration.ExistsInScoredSet(key, pieceID)
		if err != nil {
			return maskAny(err)
		}
		if !exists {
			err := s.storage.Configuration.SetElementByScore(key, pieceID, 0)
			if err != nil {
				return maskAny(err)
			}
		}
	}

	// Add the piece ID and the associated results to the local mapping.
	{
		s.mutex.Lock()
		key := pieceListKey(namespace)
		s.pieces[appendToNamespace(key, pieceID)] = results
		s.mutex.Unlock()
	}

	return nil
}

func (s *service) Delete(ctx context.Context, labels []string) error {
	namespace := labelsToNamespace(labels...)

	//
	{
		key := rulerListKey(namespace)
		err := s.storage.Configuration.Remove(key)
		if err != nil {
			return maskAny(err)
		}
	}

	//
	{
		key := pieceListKey(namespace)
		err := s.storage.Configuration.Remove(key)
		if err != nil {
			return maskAny(err)
		}
	}

	//
	{
		s.mutex.Lock()
		key := pieceListKey(namespace)
		for k, _ := range s.pieces {
			if strings.HasPrefix(k, key) {
				delete(s.pieces, k)
			}
		}
		s.mutex.Unlock()
	}

	return nil
}

func (s *service) Execute(ctx context.Context, labels []string) (string, []interface{}, error) {
	var err error

	// ruler namespace
	namespace := labelsToNamespace(labels...)

	// get current stage
	var currentStage currentstage.Value
	{
		var ok bool
		currentStage, ok = currentstage.FromContext(ctx)
		if !ok {
			return "", nil, maskAnyf(invalidContextError, "current stage must not be empty")
		}
	}

	// find a ruler (best ruler result decides about which ruler to use)
	var rulerID string
	{
		if currentStage.Trial() {
			var elements []string
			{
				//
				key := rulerListKey(namespace)
				elements, err = s.storage.Configuration.GetHighestScoredElements(key, 1)
				if err != nil {
					return "", nil, maskAny(err)
				}
			}

			//
			if len(elements) == 0 {
				rulerID = randomRuler
			} else {
				rulerID = elements[0]
			}

			//
			{
				key := rulerUsedKey(namespace)
				err := s.storage.Configuration.Set(key, rulerID)
				if err != nil {
					return "", nil, maskAny(err)
				}
			}
		}

		// handle the requested replay case
		if currentStage.Replay() {
			key := rulerUsedKey(namespace)
			rulerID, err = s.storage.Configuration.Get(key)
			if err != nil {
				return "", nil, maskAny(err)
			}
		}
	}

	//
	var pieceID string
	var results []interface{}
	{
		ruler, ok := s.rulers[rulerID]
		if !ok {
			return "", nil, maskAnyf(notFoundError, "no ruler for key: %s", rulerID)
		}
		pieceID, err = ruler(ctx, labels)
		if err != nil {
			return "", nil, maskAny(err)
		}
		err := s.storage.Configuration.Set(pieceUsedKey(namespace), pieceID)
		if err != nil {
			return "", nil, maskAny(err)
		}

		s.mutex.Lock()
		results, _ = s.pieces[pieceID]
		s.mutex.Unlock()
	}

	return pieceID, results, nil
}

func (s *service) Failure(ctx context.Context, labels []string) error {
	err := s.Delete(ctx, labels)
	if err != nil {
		return maskAny(err)
	}

	return nil
}

func (s *service) Success(ctx context.Context, labels []string) error {
	namespace := labelsToNamespace(labels...)

	//
	{
		rulerID, err := s.storage.Configuration.Get(rulerUsedKey(namespace))
		if err != nil {
			return maskAny(err)
		}
		_, err = s.storage.Configuration.IncrementScoredElement(rulerListKey(namespace), rulerID, 1)
		if err != nil {
			return maskAny(err)
		}
	}

	//
	{
		pieceID, err := s.storage.Configuration.Get(pieceUsedKey(namespace))
		if err != nil {
			return maskAny(err)
		}
		_, err = s.storage.Configuration.IncrementScoredElement(pieceListKey(namespace), pieceID, 1)
		if err != nil {
			return maskAny(err)
		}
	}

	return nil
}

func (s *service) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.closer)
	})
}
