package clickhouse

type CreateUser struct {
	DbName   string
	UserName string
	Pass     string
}

type DeleteUser struct {
	UserName string
	DbName   string
}
