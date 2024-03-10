package main

import "os"

func initSQLSchemaFile() {
	err := os.Remove("./schema.sql")
	if err != nil {
		panic(err)
	}
	quotationSchemaSQL, err := os.ReadFile("../../exchange/repository/quotation/ormandredis/schema.sql")
	if err != nil {
		panic(err)
	}
	matchingDetailSchemaSQL, err := os.ReadFile("../../exchange/repository/matching/mysqlandmq/schema.sql")
	if err != nil {
		panic(err)
	}
	orderSchemaSQL, err := os.ReadFile("../../exchange/repository/order/ormandmq/schema.sql")
	if err != nil {
		panic(err)
	}
	assetSchemaSQL, err := os.ReadFile("../../exchange/repository/asset/memoryandmysql/schema.sql")
	if err != nil {
		panic(err)
	}
	candleSchemaSQL, err := os.ReadFile("../../exchange/repository/candle/schema.sql")
	if err != nil {
		panic(err)
	}
	sequencerSchemaSQL, err := os.ReadFile("../../exchange/repository/sequencer/kafkaandmysql/schema.sql")
	if err != nil {
		panic(err)
	}
	authSchemaSQL, err := os.ReadFile("../../auth/repository/auth/mysql/schema.sql")
	if err != nil {
		panic(err)
	}
	accountSchemaSQL, err := os.ReadFile("../../auth/repository/account/mysql/schema.sql")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("./schema.sql",
		[]byte(string(quotationSchemaSQL)+
			"\n"+string(matchingDetailSchemaSQL)+
			"\n"+string(candleSchemaSQL)+
			"\n"+string(orderSchemaSQL)+
			"\n"+string(assetSchemaSQL)+
			"\n"+string(sequencerSchemaSQL)+
			"\n"+string(accountSchemaSQL)+
			"\n"+string(authSchemaSQL)),
		0644,
	)
	if err != nil {
		panic(err)
	}
}

func main() {
	initSQLSchemaFile()
}
