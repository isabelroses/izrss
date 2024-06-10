package lib

import (
	"log"
	"os"
)

func SetupLogger() {
	file, err := os.OpenFile(getStateFile("izrss.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(file)
}
