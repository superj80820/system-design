package main

import "os"

func initSQLSchemaFile() {
	err := os.Remove("./schema.sql")
	if err != nil {
		panic(err)
	}
	quotationSchemaSQL, err := os.ReadFile("../../exchange/repository/quotation/mysqlandredis/schema.sql")
	if err != nil {
		panic(err)
	}
	tradingSchemaSQL, err := os.ReadFile("../../exchange/repository/trading/mysqlandmongo/schema.sql")
	if err != nil {
		panic(err)
	}
	orderSchemaSQL, err := os.ReadFile("../../exchange/repository/order/mysql/schema.sql")
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
	authSchemaSQL, err := os.ReadFile("../../auth/repository/schema.sql")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("./schema.sql", []byte(string(quotationSchemaSQL)+"\n"+string(tradingSchemaSQL)+"\n"+string(candleSchemaSQL)+"\n"+string(orderSchemaSQL)+"\n"+string(sequencerSchemaSQL)+"\n"+string(authSchemaSQL)), 0644)
	if err != nil {
		panic(err)
	}
}

func main() {
	initSQLSchemaFile()
}
