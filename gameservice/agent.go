package GameService

type Agent struct {
	id                 int
	definitionIndex    int
	debugName          string
	posX               float32
	posY               float32
	plane              int
	facingX            float32
	facingY            float32
	modelId            int
	allegianceFlags    int
	speed              float32
	encName            string
	profession         int
	level              int
	fileId             int
	unkPropertiesBytes string
}
