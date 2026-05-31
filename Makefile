.PHONY: build build-linux build-windows dev-frontend dev-backend clean

build: build-frontend build-backend

build-frontend:
	cd frontend && npm ci && npm run build
	cp -r frontend/dist backend/embed/

build-backend:
	cd backend && CGO_ENABLED=0 go build -o ../living-recorder .

build-linux: build-frontend
	cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../living-recorder-linux-amd64 .

build-windows: build-frontend
	cd backend && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ../living-recorder-windows-amd64.exe .

dev-frontend:
	cd frontend && npm run dev

dev-backend:
	cd backend && go run .

clean:
	rm -f living-recorder living-recorder-linux-amd64 living-recorder-windows-amd64.exe
	rm -rf frontend/dist backend/embed/dist
