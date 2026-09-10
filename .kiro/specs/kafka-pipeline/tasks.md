# Implementation Plan: Kafka CI/CD Pipeline

## Overview

This implementation plan creates a Woodpecker CI/CD pipeline that validates Keploy's Kafka integration capabilities using the ecommerce sample application. The implementation consists of two primary artifacts: a pipeline configuration file and a test orchestration script that manages Docker infrastructure, Kafka setup, and Keploy record/replay operations.

## Tasks

- [x] 1. Create pipeline configuration file
  - Create `enterprise/.woodpecker/kafka-ecommerce.yml` with pipeline structure
  - Configure pipeline triggers (pull request events)
  - Set up pipeline dependencies (depends on prepare-and-run)
  - Configure Docker-in-Docker environment variables
  - Define three steps: download-artifacts, checkout-samples, run-kafka-tests
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 13.1, 13.2, 13.3, 13.4, 13.5, 13.6_

- [x] 2. Implement artifact download step
  - Configure download-artifacts step to fetch Keploy binary from MinIO
  - Configure download of enterprise Docker image tar file
  - Set up GitHub App authentication for private repository access
  - Use --no-load flag for deferred Docker image loading
  - Set executable permissions on downloaded binary
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

- [x] 3. Implement sample checkout step
  - Configure checkout-samples step to clone ecommerce_sample_app repository
  - Use shallow clone with depth 1 for performance
  - Target go-services subdirectory
  - Set step dependency on artifact download completion
  - _Requirements: 3.1, 3.2, 3.3, 3.4_

- [x] 4. Implement Docker environment setup step
  - Configure run-kafka-tests step with privileged mode
  - Mount /sys/kernel/debug and /sys/fs/bpf volumes for eBPF support
  - Start Docker daemon with registry mirror configuration
  - Add wait logic for Docker readiness
  - Load enterprise Docker image from tar file
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6_

- [x] 5. Create test script skeleton
  - Create `enterprise/.ci/scripts/kafka-ecommerce.sh` with bash shebang
  - Add script header comments documenting entry point and dependencies
  - Set up error handling with `set -e` and trap for cleanup
  - Define exit code constants
  - _Requirements: 12.1, 12.2, 12.6_

- [ ] 6. Implement helper functions
  - [x] 6.1 Implement check_command_success function
    - Validate previous command exit code
    - Log error message on failure
    - Call cleanup and exit with code 1
    - _Requirements: 12.3, 14.1_
  
  - [x] 6.2 Implement wait_for_port function
    - Accept host, port, and timeout parameters
    - Loop with timeout checking TCP port availability
    - Return 0 if ready, 1 if timeout
    - _Requirements: 5.6, 12.7_
  
  - [x] 6.3 Implement wait_for_kafka function
    - Use kafka-topics command to verify broker health
    - Implement timeout with error logging
    - Display Kafka logs on timeout
    - Exit with error if Kafka fails to start
    - _Requirements: 5.6, 5.7, 12.7_
  
  - [x] 6.4 Implement wait_for_mysql function
    - Accept container name and timeout parameters
    - Use mysqladmin ping for health check
    - Display container logs on timeout
    - Exit with error if MySQL fails to start
    - _Requirements: 6.4, 12.7_
  
  - [x] 6.5 Implement cleanup_resources function
    - Stop all Docker containers (order_service, product_service, user_service, mysql-*, kafka, zookeeper)
    - Remove all Docker containers
    - Remove keploy-network
    - Log cleanup progress
    - _Requirements: 12.5, 14.5, 14.6_

- [x] 7. Implement infrastructure setup
  - [x] 7.1 Create Docker network
    - Create keploy-network using docker network create
    - Add error checking after network creation
    - _Requirements: 5.1, 14.1_
  
  - [x] 7.2 Start Zookeeper container
    - Run confluentinc/cp-zookeeper:7.5.0 container
    - Configure ZOOKEEPER_CLIENT_PORT=2181
    - Configure ZOOKEEPER_TICK_TIME=2000
    - Attach to keploy-network
    - Expose port 2181
    - Add error checking after container start
    - _Requirements: 5.2, 14.1_
  
  - [x] 7.3 Start Kafka broker container
    - Run confluentinc/cp-kafka:7.5.0 container
    - Configure KAFKA_BROKER_ID=1
    - Configure KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181
    - Configure KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://kafka:9092
    - Configure KAFKA_AUTO_CREATE_TOPICS_ENABLE=true
    - Configure KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1
    - Attach to keploy-network
    - Expose port 9092
    - Add error checking after container start
    - _Requirements: 5.3, 5.4, 14.1_
  
  - [x] 7.4 Wait for Kafka readiness and create topic
    - Call wait_for_kafka function
    - Create order-events topic with 3 partitions using kafka-topics command
    - Verify topic creation succeeded
    - _Requirements: 5.5, 5.6, 5.7_
  
  - [x] 7.5 Start MySQL containers
    - Start mysql-users container (port 3307, user_db database)
    - Start mysql-products container (port 3308, product_db database)
    - Start mysql-orders container (port 3309, order_db database)
    - Mount SQL initialization scripts for each database
    - Configure root password and user credentials
    - Attach all containers to keploy-network
    - _Requirements: 6.1, 6.2, 6.3_
  
  - [x] 7.6 Wait for MySQL readiness
    - Call wait_for_mysql for each MySQL container
    - Verify all databases are healthy before proceeding
    - _Requirements: 6.4_

- [x] 8. Implement microservices deployment
  - [x] 8.1 Build and start user_service
    - Build user_service Docker image from Dockerfile
    - Run container on port 8082
    - Configure DB_HOST=mysql-users environment variable
    - Configure database credentials
    - Attach to keploy-network
    - Add error checking after build and start
    - _Requirements: 7.1, 7.7, 14.1_
  
  - [x] 8.2 Build and start product_service
    - Build product_service Docker image from Dockerfile
    - Run container on port 8081
    - Configure DB_HOST=mysql-products environment variable
    - Configure database credentials
    - Attach to keploy-network
    - Add error checking after build and start
    - _Requirements: 7.2, 7.7, 14.1_
  
  - [x] 8.3 Build and start order_service
    - Build order_service Docker image from Dockerfile
    - Run container on port 8080
    - Configure DB_HOST=mysql-orders environment variable
    - Configure KAFKA_BROKERS=kafka:9092 environment variable
    - Configure KAFKA_TOPIC=order-events environment variable
    - Configure KAFKA_GROUP_ID=order-service-group environment variable
    - Configure USER_SERVICE_URL and PRODUCT_SERVICE_URL
    - Attach to keploy-network
    - Add error checking after build and start
    - _Requirements: 7.3, 7.4, 7.5, 7.6, 7.7, 14.1_
  
  - [x] 8.4 Wait for microservices readiness
    - Wait for user_service port 8082
    - Wait for product_service port 8081
    - Wait for order_service port 8080
    - _Requirements: 7.7_

- [x] 9. Implement Keploy record mode
  - [x] 9.1 Execute Keploy in record mode
    - Run keployE binary with record command
    - Target order_service container
    - Set --generateGithubActions=false flag
    - Enable --debug flag
    - Redirect output to record_logs.txt
    - Run in background to allow test requests
    - _Requirements: 8.1, 8.2_
  
  - [x] 9.2 Send test HTTP requests
    - Wait for order_service to be ready on port 8080
    - Send POST request to create order (user_id=1, product_id=1, quantity=2)
    - Send POST request to create order (user_id=2, product_id=2, quantity=1)
    - Send POST request to create order (user_id=1, product_id=3, quantity=5)
    - Verify HTTP responses are successful
    - _Requirements: 8.3, 8.5, 10.1_
  
  - [x] 9.3 Wait for Kafka event processing
    - Sleep for 10 seconds to allow Kafka producer and consumer operations
    - _Requirements: 8.4, 8.6_
  
  - [x] 9.4 Stop Keploy recording
    - Send termination signal to Keploy process
    - Wait for Keploy to finish writing test cases and mocks
    - _Requirements: 8.7_
  
  - [ ]* 9.5 Validate recording output
    - Check for "ERROR" in record_logs.txt and exit if found
    - Check for "WARNING: DATA RACE" in record_logs.txt and exit if found
    - Verify keploy directory exists and contains test cases
    - Verify mocks directory contains Kafka mocks
    - _Requirements: 8.8, 8.9, 10.2, 10.3_

- [x] 10. Implement Keploy test mode
  - [x] 10.1 Clean up and restart infrastructure
    - Stop all running containers
    - Restart Zookeeper, Kafka, and MySQL containers
    - Recreate order-events topic
    - Wait for all infrastructure to be ready
    - _Requirements: 9.1, 14.2_
  
  - [x] 10.2 Rebuild and restart microservices
    - Rebuild and start user_service, product_service, order_service
    - Wait for all services to be ready
    - _Requirements: 9.2_
  
  - [x] 10.3 Execute Keploy in test mode
    - Run keployE binary with test command
    - Target order_service container
    - Set --delay 30 flag for replay timing
    - Set --generateGithubActions=false flag
    - Set --disableMockUpload flag
    - Redirect output to test_logs.txt
    - Capture exit code
    - _Requirements: 9.3, 9.4, 9.5_
  
  - [ ]* 10.4 Validate test output
    - Check exit code and exit if non-zero
    - Check for "ERROR" in test_logs.txt and exit if found
    - Check for "WARNING: DATA RACE" in test_logs.txt and exit if found
    - Verify test pass/fail status is reported
    - _Requirements: 9.7, 9.8, 9.9, 9.10, 10.4, 10.5_

- [ ]* 11. Implement comprehensive validation
  - Verify Kafka events were produced to order-events topic
  - Verify order service consumer received events
  - Verify Kafka message serialization succeeded
  - Verify Kafka compression handling (if enabled)
  - _Requirements: 10.2, 10.3, 10.6, 10.7_

- [x] 12. Implement error handling and logging
  - [x] 12.1 Add error checking after all critical commands
    - Add check_command_success calls after Docker operations
    - Add check_command_success calls after Kafka operations
    - Add check_command_success calls after service builds
    - _Requirements: 12.3, 14.1_
  
  - [x] 12.2 Implement timeout handling
    - Add timeout logic to all wait functions
    - Display appropriate error messages on timeout
    - Call cleanup_resources on timeout
    - _Requirements: 5.7, 12.7, 14.2_
  
  - [x] 12.3 Add logging throughout script
    - Add success messages (✅) for completed steps
    - Add informational messages (📦) for progress
    - Add waiting messages (⏳) for long operations
    - Add error messages (ERROR:) for failures
    - _Requirements: 12.4_
  
  - [x] 12.4 Implement cleanup on failure
    - Register cleanup_resources function with trap EXIT
    - Ensure cleanup happens on any script exit
    - Preserve logs for debugging on failure
    - _Requirements: 14.3, 14.4_

- [x] 13. Configure environment variables
  - Add KEPLOY_CI_API_KEY from secrets to pipeline
  - Add GITHUB_APP_PRIVATE_KEY from secrets to pipeline
  - Set GOPRIVATE environment variable for private Go modules
  - Configure DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH
  - _Requirements: 11.1, 11.2, 11.3, 11.5_

- [x] 14. Checkpoint - Test pipeline locally
  - Ensure all files are created and have correct permissions
  - Verify script syntax is correct (shellcheck)
  - Test Docker network creation and cleanup
  - Test helper functions work correctly
  - Ask the user if questions arise

- [x] 15. Final integration and testing
  - Verify pipeline triggers on pull request
  - Verify artifacts download successfully
  - Verify infrastructure starts correctly
  - Verify Keploy recording captures Kafka interactions
  - Verify Keploy testing replays correctly
  - Verify cleanup happens on success and failure
  - _Requirements: All_

## Notes

- Tasks marked with `*` are optional validation tasks that can be skipped for faster MVP
- The pipeline follows the same structure as sqs-localstack.yml for consistency
- All Docker operations use the keploy-network for container communication
- Error handling is critical - the script must exit cleanly on any failure
- Logs are preserved for debugging in CI artifacts
- The test script orchestrates a complex multi-container environment with proper dependency ordering
