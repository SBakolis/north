package knowledge

type Snapshot struct {
	Provider           string
	SourceRoot         string
	Change             *ChangeDescription
	Requirements       []Requirement
	DesignDecisions    []DesignDecision
	Tasks              []Task
	AcceptanceCriteria []Criterion
	References         []Reference
	RawArtifacts       []ArtifactReference
}

type ChangeDescription struct{ Title, Summary string }
type Requirement struct{ ID, Text, Source string }
type DesignDecision struct{ ID, Text, Source string }
type Task struct {
	ID, Text, Source string
	Completed        bool
}
type Criterion struct{ ID, Text, Source string }
type Reference struct{ Title, URI string }
type ArtifactReference struct{ Path, SHA256 string }
