
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
            }
        }

        stage('Configuration Validation') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" config --quiet'
            }
        }

        stage('Build Images') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" build'
            }
        }

        stage('Start Services') {
            steps {
                bat 'docker compose -f "%COMPOSE_FILE%" up -d'
            }
        }

        stage('Automated Testing') {
            steps {
                bat 'docker compose exec -T user_service python -m pytest /app/tests/test_user.py -v'
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
        }

        failure {
            echo 'CI pipeline failed. Check Console Output.'
        }

        always {
            script {
                try {
                    bat 'docker compose -f "%COMPOSE_FILE%" down'
                    echo 'Docker Compose cleanup completed.'
                } catch (Exception e) {
                    echo "Cleanup skipped or failed: ${e.getMessage()}"
                }
            }

            echo 'Pipeline execution finished.'
        }
    }
}
