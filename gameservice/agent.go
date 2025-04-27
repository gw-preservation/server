package GameService

type Agent struct {
	id                 int
	posX               float32
	posY               float32
	plane              int
	facingX            float32
	facingY            float32
	modelId            int
	agentType          int
	modelType          int
	speed              float32
	encName            string
	profession         int
	level              int
	fileId             int
	unkPropertiesBytes string
}
