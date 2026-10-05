# Requirements Document

## Introduction

This document specifies the requirements for a CI/CD pipeline that tests Kafka integration with record and replay functionality in the enterprise project. The pipeline will use the ecommerce_sample_app's order_service (which uses Kafka for event streaming) to validate that Keploy can correctly record and replay Kafka interactions in a CI environment.

## Glossary

- **Pipeline**: The Woodpecker CI/CD pipeline configuration file
- **Keploy_Binary**: The enterprise Keploy binary artifact (keployE)
- **Docker_Image**: The enterprise Docker image artifact
- **Sample_App**: The ecommerce_sample_app Go services application
- **Order_Service**: The Go-based order service that uses Kafka for event streaming
- **Kafka_Broker**: Apache Kafka message broker (confluentinc/cp-kafka)
- **Zookeeper**: Apache Zookeeper service required by Kafka
- **MySQL_Database**: MySQL database instances for the microservices
- **Test_Script**: Shell script that orchestrates the Kafka integration test
- **Artifact_Downloader**: CI step that downloads pre-built binaries and images from MinIO
- **Docker_Daemon**: Docker-in-Docker service for running containers in CI
- **Keploy_Network**: Docker network for container communication

## Requirements

### Requirement 1: Pipeline Configuration

**User Story:** As a CI/CD engineer, I want a Woodpecker pipeline configuration for Kafka testing, so that Kafka integration is automatically validated on pull requests.

#### Acceptance Criteria

1. THE Pipeline SHALL be triggered on pull request events
2. THE Pipeline SHALL depend on the prepare-and-run pipeline
3. THE Pipeline SHALL use the keploy-ci Docker images (ghcr.io/keploy/keploy-ci:1.2.10 and slim variant)
4. THE Pipeline SHALL target linux/amd64 platform
5. THE Pipeline SHALL use shallow git clone with depth 1 and lfs disabled

### Requirement 2: Artifact Download

**User Story:** As a CI/CD engineer, I want to download pre-built Keploy artifacts, so that the pipeline can test the current build.

#### Acceptance Criteria

1. THE Artifact_Downloader SHALL download the Keploy binary from MinIO using the CI commit SHA
2. THE Artifact_Downloader SHALL download the enterprise Docker image tar file from MinIO
3. THE Artifact_Downloader SHALL use the --no-load flag to defer Docker image loading
4. THE Artifact_Downloader SHALL authenticate using GitHub App credentials
5. THE Artifact_Downloader SHALL set executable permissions on the downloaded binary

### Requirement 3: Sample Application Checkout

**User Story:** As a CI/CD engineer, I want to checkout the ecommerce sample application, so that I can run Kafka integration tests.

#### Acceptance Criteria

1. THE Pipeline SHALL clone the ecommerce_sample_app repository from GitHub
2. THE Pipeline SHALL use shallow clone with depth 1 for faster checkout
3. THE Pipeline SHALL checkout the go-services subdirectory containing the Kafka-enabled order service
4. THE Pipeline SHALL execute after artifact download completes

### Requirement 4: Docker Environment Setup

**User Story:** As a CI/CD engineer, I want Docker-in-Docker capability, so that I can run containerized services for testing.

#### Acceptance Criteria

1. THE Pipeline SHALL run with privileged mode enabled for Docker-in-Docker
2. THE Pipeline SHALL mount /sys/kernel/debug and /sys/fs/bpf volumes for eBPF support
3. THE Pipeline SHALL start the Docker daemon using the start-docker command
4. THE Pipeline SHALL wait for Docker to be ready before proceeding
5. THE Pipeline SHALL use the registry mirror at http://192.168.116.165:5000 for faster image pulls
6. THE Pipeline SHALL load the enterprise Docker image from the tar file

### Requirement 5: Kafka Infrastructure Setup

**User Story:** As a developer, I want Kafka and its dependencies running in Docker, so that the order service can publish and consume events.

#### Acceptance Criteria

1. THE Test_Script SHALL create a Docker network named keploy-network
2. THE Test_Script SHALL start Zookeeper container on port 2181
3. THE Test_Script SHALL start Kafka broker container with PLAINTEXT listener on port 9092
4. THE Test_Script SHALL configure Kafka with auto-create topics enabled
5. THE Test_Script SHALL create the order-events topic with 3 partitions
6. THE Test_Script SHALL wait for Kafka to be healthy before proceeding
7. WHEN Kafka fails to start within timeout, THEN THE Test_Script SHALL exit with error

### Requirement 6: MySQL Database Setup

**User Story:** As a developer, I want MySQL databases for each microservice, so that the application can persist data.

#### Acceptance Criteria

1. THE Test_Script SHALL start three MySQL 8.0 containers (mysql-users, mysql-products, mysql-orders)
2. THE Test_Script SHALL configure each MySQL instance with appropriate database names and credentials
3. THE Test_Script SHALL initialize databases using SQL scripts from the sample app
4. THE Test_Script SHALL wait for MySQL health checks to pass before starting services
5. THE Test_Script SHALL expose MySQL on ports 3307, 3308, and 3309 respectively

### Requirement 7: Microservices Deployment

**User Story:** As a developer, I want all microservices running, so that I can test the complete order flow with Kafka.

#### Acceptance Criteria

1. THE Test_Script SHALL build and start the user_service container on port 8082
2. THE Test_Script SHALL build and start the product_service container on port 8081
3. THE Test_Script SHALL build and start the order_service container on port 8080
4. THE Test_Script SHALL configure order_service with Kafka broker address (kafka:9092)
5. THE Test_Script SHALL configure order_service with topic name (order-events)
6. THE Test_Script SHALL configure order_service with consumer group ID
7. THE Test_Script SHALL ensure services start in dependency order (databases → user/product → order)

### Requirement 8: Keploy Record Mode

**User Story:** As a developer, I want to record Kafka interactions, so that I can replay them in tests.

#### Acceptance Criteria

1. THE Test_Script SHALL execute Keploy in record mode using the keployE binary
2. THE Test_Script SHALL target the order_service container for recording
3. THE Test_Script SHALL send HTTP requests to create orders (which trigger Kafka events)
4. THE Test_Script SHALL wait for Kafka producer and consumer interactions to be recorded
5. THE Test_Script SHALL capture at least 3 order creation requests with different data
6. THE Test_Script SHALL record both Kafka produce and consume operations
7. WHEN recording completes, THEN THE Test_Script SHALL generate test cases and mocks
8. IF "ERROR" appears in record logs, THEN THE Test_Script SHALL exit with failure
9. IF "WARNING: DATA RACE" appears in record logs, THEN THE Test_Script SHALL exit with failure

### Requirement 9: Keploy Test Mode

**User Story:** As a developer, I want to replay recorded Kafka interactions, so that I can verify Kafka integration works correctly.

#### Acceptance Criteria

1. THE Test_Script SHALL stop all running containers after recording
2. THE Test_Script SHALL restart infrastructure (Kafka, Zookeeper, MySQL) for test mode
3. THE Test_Script SHALL execute Keploy in test mode using the keployE binary
4. THE Test_Script SHALL replay recorded test cases with a 30-second delay
5. THE Test_Script SHALL disable mock upload during testing
6. THE Test_Script SHALL verify that Kafka mocks are correctly replayed
7. WHEN test mode completes, THEN THE Test_Script SHALL report pass/fail status
8. IF "ERROR" appears in test logs, THEN THE Test_Script SHALL exit with failure
9. IF "WARNING: DATA RACE" appears in test logs, THEN THE Test_Script SHALL exit with failure
10. IF test exit code is non-zero, THEN THE Test_Script SHALL exit with failure

### Requirement 10: Test Validation

**User Story:** As a developer, I want comprehensive test validation, so that I can trust the Kafka integration works.

#### Acceptance Criteria

1. THE Test_Script SHALL verify that order creation requests succeed during recording
2. THE Test_Script SHALL verify that Kafka events are produced to the order-events topic
3. THE Test_Script SHALL verify that the order service consumer receives events
4. THE Test_Script SHALL verify that replayed tests match recorded behavior
5. THE Test_Script SHALL check for race conditions in both record and test modes
6. THE Test_Script SHALL validate that no errors occur during Kafka message serialization
7. THE Test_Script SHALL validate that Kafka compression (if enabled) is handled correctly

### Requirement 11: Environment Configuration

**User Story:** As a CI/CD engineer, I want proper environment configuration, so that the pipeline has necessary credentials and settings.

#### Acceptance Criteria

1. THE Pipeline SHALL provide KEPLOY_CI_API_KEY from secrets
2. THE Pipeline SHALL provide GITHUB_APP_PRIVATE_KEY from secrets for private repository access
3. THE Pipeline SHALL set GOPRIVATE environment variable for private Go modules
4. THE Pipeline SHALL configure Docker daemon flags for registry mirror
5. THE Pipeline SHALL set DOCKER_HOST, DOCKER_TLS_VERIFY, and DOCKER_CERT_PATH for Docker-in-Docker

### Requirement 12: Test Script Creation

**User Story:** As a CI/CD engineer, I want a dedicated test script for Kafka testing, so that the test logic is maintainable and reusable.

#### Acceptance Criteria

1. THE Test_Script SHALL be located at .ci/scripts/kafka-ecommerce.sh
2. THE Test_Script SHALL follow the same structure as sqs-localstack.sh
3. THE Test_Script SHALL include error checking after each critical command
4. THE Test_Script SHALL provide clear logging output for debugging
5. THE Test_Script SHALL clean up Docker resources on exit
6. THE Test_Script SHALL return exit code 0 on success and non-zero on failure
7. THE Test_Script SHALL handle timeouts gracefully with appropriate error messages

### Requirement 13: Pipeline File Creation

**User Story:** As a CI/CD engineer, I want a pipeline file in the woodpecker directory, so that the CI system can execute Kafka tests.

#### Acceptance Criteria

1. THE Pipeline SHALL be named kafka-ecommerce.yml
2. THE Pipeline SHALL be located in enterprise/.woodpecker/
3. THE Pipeline SHALL follow the same structure as sqs-localstack.yml
4. THE Pipeline SHALL include steps: download-artifacts, checkout-samples, run-kafka-tests
5. THE Pipeline SHALL use appropriate dependencies between steps
6. THE Pipeline SHALL include comments explaining the Kafka-specific setup

### Requirement 14: Error Handling and Cleanup

**User Story:** As a CI/CD engineer, I want proper error handling and cleanup, so that failed tests don't leave resources hanging.

#### Acceptance Criteria

1. WHEN any Docker command fails, THEN THE Test_Script SHALL log the error and exit
2. WHEN Kafka fails to start, THEN THE Test_Script SHALL stop all containers and exit
3. WHEN recording fails, THEN THE Test_Script SHALL clean up containers before exiting
4. WHEN testing fails, THEN THE Test_Script SHALL preserve logs for debugging
5. THE Test_Script SHALL stop and remove all Docker containers at the end
6. THE Test_Script SHALL remove the keploy-network after tests complete
