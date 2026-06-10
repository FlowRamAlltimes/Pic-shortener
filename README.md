# Pic-shortener

A simple REST API built in Go for uploading, dynamic resizing, and caching JPEG images, equipped with a full monitoring stack (Prometheus + Grafana).

## Arch

<img width="1118" height="660" alt="image" src="https://github.com/user-attachments/assets/8b2417e2-a486-4aea-9475-d952ed8412f0" />


## Features

- **Image Upload:** Upload original images via `POST` requests.
- **Dynamic Resizing:** Request images with custom width and quality parameters via `GET` requests.
- **In-Memory Caching:** Fast response times for duplicate requests using an in-memory cache with thread-safe `sync.RWMutex`.
- **Monitoring:** Built-in Prometheus metrics handler tracking Go runtime states and custom application behavior (cache hits and request durations).

## Tech Stack

- **Backend:** Go (Golang)
- **Database:** PostgreSQL
- **Monitoring:** Prometheus & Grafana
- **Infrastructure:** Docker & Docker Compose

## Getting Started

### Prerequisites

Make sure you have Docker and Docker Compose installed on your system.

### Running the Project

1. Clone the repository and navigate to the project directory.
2. Start the entire stack using Docker Compose:

```bash
sudo docker compose up --build -d
```

This command builds the Go application and starts all services in the background.
Service Ports

Once running, the following services will be available:

    Go Backend API: http://localhost:10000

    Prometheus UI: http://localhost:9090

    Grafana Dashboards: http://localhost:3000

API Endpoints
1. Upload Image

    URL: /images

    Method: POST

    Content-Type: multipart/form-data

    Body: image (file)

    Response: Success message with the image file HASH.

2. Get / Resize Image

    URL: /images

    Method: GET

    Query Parameters:

        hash: The unique hash of the uploaded image.

        width: Target width in pixels.

        quality: JPEG quality (1-100).

    Response: The processed JPEG image.

3. Metrics

    URL: /metrics

    Method: GET

    Description: Exposes standard Go runtime metrics and custom application metrics for Prometheus scraping.
# Grafana Dashboard
<img width="1524" height="818" alt="image" src="https://github.com/user-attachments/assets/e99c6f09-fa1f-471c-a187-8bb58d98d735" />
