

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

        MYSQL_USERS_PORT = '3417'
        MYSQL_PRODUCTS_PORT = '3418'
        MYSQL_ORDERS_PORT = '3419'

        USER_SERVICE_PORT = '18092'
        PRODUCT_SERVICE_PORT = '18091'
        ORDER_SERVICE_PORT = '18094'
        APIGATEWAY_PORT = '18093'

        LOCALSTACK_PORT = '14566'

        AWS_CLI = 'C:\\Program Files\\Amazon\\AWSCLIV2\\aws.exe'
        AWS_REGION = 'ap-southeast-2'
        ECR_REGISTRY = '305846488272.dkr.ecr.ap-southeast-2.amazonaws.com'
    }

    stages {

        stage('Checkout') {
            steps {
                checkout scm

                script {
                    env.GIT_COMMIT_SHORT = bat(
                        returnStdout: true,
                        script: '@echo off\ngit rev-parse --short=12 HEAD'
                    ).trim()

                    echo "Commit tag: ${env.GIT_COMMIT_SHORT}"
                }
            }
        }

        stage('Tool Validation') {
            steps {
                bat 'git --version'
                bat 'docker --version'
                bat 'docker compose version'
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

        stage('Push Images to ECR') {
            steps {
                withCredentials([
                    usernamePassword(
                        credentialsId: 'aws-ecr-credentials',
                        usernameVariable: 'AWS_ACCESS_KEY_ID',
                        passwordVariable: 'AWS_SECRET_ACCESS_KEY'
                    )
                ]) {
                    script {
                        bat '''
                            @echo off
                            set AWS_DEFAULT_REGION=%AWS_REGION%
                            "%AWS_CLI%" ecr get-login-password --region %AWS_REGION% | docker login --username AWS --password-stdin %ECR_REGISTRY%
                            if errorlevel 1 exit /b 1
                        '''

                        def services = [
                            [compose: 'user_service', repo: 'user-service'],
                            [compose: 'product_service', repo: 'product-service'],
                            [compose: 'order_service', repo: 'order-service'],
                            [compose: 'apigateway', repo: 'apigateway']
                        ]

                        services.each { service ->
                            echo "Publishing ${service.repo}"

                            def imageId = bat(
                                returnStdout: true,
                                script: "@echo off\ndocker compose -f \"%COMPOSE_FILE%\" images -q ${service.compose}"
                            ).trim()

                            if (!imageId) {
                                error "No Docker image found for ${service.compose}"
                            }

                            def ecrImage = "${env.ECR_REGISTRY}/${service.repo}:${env.GIT_COMMIT_SHORT}"

                            bat "docker tag ${imageId} ${ecrImage}"
                            bat "docker push ${ecrImage}"

                            echo "Successfully pushed ${ecrImage}"
                        }
                    }
                }
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
            echo 'CI/CD image publishing pipeline completed successfully.'

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
            echo 'Pipeline failed. Collecting diagnostic information.'

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
