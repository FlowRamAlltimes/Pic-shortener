# Pic-shortener

A simple stateless REST API built in Go for uploading, dynamic resizing, and caching JPEG, PNG and WebP images, equipped with a full monitoring stack (Prometheus + Grafana).

## Arch

<img width="849" height="520" alt="image" src="https://github.com/user-attachments/assets/25d9b355-31db-43dd-b531-869129b4b3c0" />

## Features

- **Image Upload:** Upload original images via `POST` requests.
- **Dynamic Resizing:** Request images with custom width and quality parameters via `GET` requests.
- **In-Memory Caching:** Fast response times for duplicate requests using an in-memory cache with thread-safe `sync.RWMutex`.
- **Monitoring:** Built-in Prometheus metrics handler tracking Go runtime states and custom application behavior (cache hits and request durations).
- **Storage:** S3 storage which provides the possibility to make simple CDN service and group some services

## Tech Stack

- **Backend:** Go (Golang)
- **Database:** PostgreSQL
- **Monitoring:** Prometheus & Grafana
- **Infrastructure:** Docker & Docker Compose
- **Storage:** MinIO S3 API

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

    PostgeSQL Database: http://localhost:5432

    Prometheus UI: http://localhost:9090

    Grafana Dashboards: http://localhost:3000

    MinIO Storage Client: http://localhost:9000

    MinIO Storage UI: http://localhost:9001

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

   * hash: The unique hash of the uploaded image.

   * width: Target width in pixels.

   * quality: JPEG quality (1-100).

   * format: webp, jpeg or png

    Response: The processed WebP, PNG or JPEG image.

4. Metrics

    URL: /metrics

    Method: GET

    Description: Exposes standard Go runtime metrics and custom application metrics for Prometheus scraping.
# Grafana Result Example
<img width="1849" height="1029" alt="image" src="https://github.com/user-attachments/assets/befebe59-e1b5-49fa-b630-6f5c77af96e2" />


