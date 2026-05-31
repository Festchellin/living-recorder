.PHONY: build build-native build-embed build-linux build-windows dev-frontend dev-backend clean

build: build-native

build-native: build-frontend build-native-linux build-native-windows
build-embed: build-frontend build-embed-linux build-embed-windows

build-frontend:
	cd frontend && npm ci && npm run build
	cp -r frontend/dist backend/embed/

build-native-linux: build-frontend
	cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../living-recorder-linux-amd64 .

build-native-windows: build-frontend
	cd backend && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ../living-recorder-windows-amd64.exe .

build-embed-linux: build-frontend
	cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags embedffmpeg -o ../living-recorder-linux-amd64-embed-ffmpeg .

build-embed-windows: build-frontend
	cd backend && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags embedffmpeg -o ../living-recorder-windows-amd64-embed-ffmpeg.exe .

dev-frontend:
	cd frontend && npm run dev

dev-backend:
	cd backend && go run .

clean:
	rm -f living-recorder living-recorder-linux-amd64 living-recorder-windows-amd64.exe
	rm -f living-recorder-linux-amd64-embed-ffmpeg living-recorder-windows-amd64-embed-ffmpeg.exe
	rm -rf frontend/dist backend/embed/dist
