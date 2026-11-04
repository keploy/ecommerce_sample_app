
pipeline {
    agent any

    options {
        timestamps()
        disableConcurrentBuilds()
    }

    triggers {
        pollSCM('H/5 * * * *')
    }

    environment {
        COMPOSE_FILE = 'docker-compose.yml'
        IMAGE_TAG = "build-${BUILD_NUMBER}"

        // Separate ports for Jenkins Docker Compose stack
        MYSQL_USERS_PORT = '3417'
        MYSQL_PRODUCTS_PORT = '3418'
        MYSQL_ORDERS_PORT = '3419'

        USER_SERVICE_PORT = '18092'
        PRODUCT_SERVICE_PORT = '18091'
        ORDER_SERVICE_PORT = '18094'
        APIGATEWAY_PORT = '18093'

        LOCALSTACK_PORT = '14566'

        // AWS CLI
        AWS_CLI = 'C:\\Program Files\\Amazon\\AWSCLIV2\\aws.exe'
    }

    stages {

        stage('Checkout') {
            steps {
                checkout scm
                bat 'git rev-parse --short HEAD'
            }
        }

        stage('Tool Validation') {
            steps {
                bat 'git --version'
                bat 'docker --version'
                bat 'docker compose version'

                // Verify AWS CLI is accessible to Jenkins service
                bat '"%AWS_CLI%" --version'
            }
        }

        stage('Configuration Validation') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" config --quiet'
                echo 'Docker Compose configuration is valid.'
            }
        }

        stage('Build Images') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" build'
                echo 'Docker images built successfully.'
            }
        }

        stage('Start Services') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" up -d'
                echo 'Docker Compose services started.'
            }
        }

        stage('Container Status Check') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" ps -a'
                bat 'docker compose -f "%COMPOSE_FILE%" logs --no-color --tail=50 user_service'
            }
        }

        stage('Automated Testing') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" exec -T user_service python -m pytest /app/tests/test_user.py -v'
                echo 'User Service automated tests passed.'
            }
        }

        stage('Container Verification') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" ps'
                bat 'docker image ls'
            }
        }

        stage('Application Logs') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" logs --no-color --tail=50'
            }
        }
    }

    post {
        success {
            echo 'CI pipeline completed successfully.'

            script {
                try {
                    bat 'docker compose -f "%COMPOSE_FILE%" down'
                    echo 'Docker Compose cleanup completed.'
                } catch (Exception e) {
                    echo "Cleanup failed: ${e.getMessage()}"
                }
            }
        }

        failure {
            echo 'CI pipeline failed. Collecting diagnostic information.'

            script {
                try {
                    bat 'docker compose -f "%COMPOSE_FILE%" ps -a'
                    bat 'docker compose -f "%COMPOSE_FILE%" logs --no-color --tail=100 user_service'
                    bat 'docker compose -f "%COMPOSE_FILE%" logs --no-color --tail=50 mysql-users'
                    bat 'docker compose -f "%COMPOSE_FILE%" logs --no-color --tail=50 mysql-products'
                    bat 'docker compose -f "%COMPOSE_FILE%" logs --no-color --tail=50 mysql-orders'
                } catch (Exception e) {
                    echo "Diagnostic collection failed: ${e.getMessage()}"
                }
            }

            echo 'Failed containers are preserved for inspection.'
        }

        always {
            echo 'Pipeline execution finished.'
        }
    }
}
