package models

type Customer struct {
	ID    int
	Email string
}

func (c Customer) Display() string {
	return c.Email
}
