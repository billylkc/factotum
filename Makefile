linux: main.go
	go build -o bin/factotum main.go

windows: main.go
	GOOS=windows GOARCH=386 go build -o bin/exe.factotum main.go
