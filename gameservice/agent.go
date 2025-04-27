package GameService

type Agent struct {
	agentId             int
	definitionIndex     int
	isPlayer            bool
	name                string
	posX                float32
	posY                float32
	plane               int
	facingX             float32
	facingY             float32
	modelId             int
	allegianceFlags     int
	speed               float32
	encName             string
	primaryProfession   int
	secondaryProfession int
	level               int
	fileId              int
	unkPropertiesBytes  string
	uuid                uint64
}
