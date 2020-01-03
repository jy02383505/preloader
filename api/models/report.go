package models

type ReportPackage struct {
	Rid string `msgpack:"rid"`
	Uid string `msgpack:"uid"`
	Sid string `msgpack:"sid"`
	Host string `msgpack:"host"`
	Status string `msgpack:"status"`
	CheckType string `msgpack:"check_type"`
	CheckValue string `msgpack:"check_value"`
	Rate string  `msgpack:"rate"`
}
