package notification

// Repository is a placeholder for services that only react to Kafka events.
type Repository struct{}

// NewRepository builds the placeholder repository used by generated bootstrap code.
func NewRepository(_ any) *Repository {
	return &Repository{}
}
