package api

import "cross-file-rename/models"

func Greeting(c models.Customer) string {
	return "hello " + c.Display()
}
