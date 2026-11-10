

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

        MYSQL_USERS_PORT = '3617'
        MYSQL_PRODUCTS_PORT = '3618'
        MYSQL_ORDERS_PORT = '3619'

        USER_SERVICE_PORT = '18192'
        PRODUCT_SERVICE_PORT = '18191'
        ORDER_SERVICE_PORT = '18194'
        APIGATEWAY_PORT = '18193'

        LOCALSTACK_PORT = '14666'

        AWS_CLI = 'C:\\Program Files\\Amazon\\AWSCLIV2\\aws.exe'
        AWS_REGION = 'ap-southeast-2'
        ECR_REGISTRY = '305846488272.dkr.ecr.ap-southeast-2.amazonaws.com'
        EC2_INSTANCE_ID = 'i-04c73c04bdd6d9e53'
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

                    env.DEPLOY_IMAGE_TAG = "${env.GIT_COMMIT_SHORT}-${env.BUILD_NUMBER}"
                    echo "Commit tag: ${env.GIT_COMMIT_SHORT}"
                    echo "Unique deployment image tag: ${env.DEPLOY_IMAGE_TAG}"
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

                            def ecrImage = "${env.ECR_REGISTRY}/${service.repo}:${env.DEPLOY_IMAGE_TAG}"

                            bat "docker tag ${imageId} ${ecrImage}"
                            bat "docker push ${ecrImage}"

                            echo "Successfully pushed ${ecrImage}"
                        }
                    }
                }
            }
        }

        stage('SSM Connectivity Test') {
            steps {
                withCredentials([
                    usernamePassword(
                        credentialsId: 'aws-ecr-credentials',
                        usernameVariable: 'AWS_ACCESS_KEY_ID',
                        passwordVariable: 'AWS_SECRET_ACCESS_KEY'
                    )
                ]) {
                    script {
                        def commandId = bat(
                            returnStdout: true,
                            script: '''
                                @echo off
                                set AWS_DEFAULT_REGION=%AWS_REGION%
                                "%AWS_CLI%" ssm send-command --region %AWS_REGION% --instance-ids %EC2_INSTANCE_ID% --document-name AWS-RunShellScript --parameters "commands=echo SSM_CONNECTIVITY_OK" --query "Command.CommandId" --output text
                            '''
                        ).trim()

                        if (!commandId || commandId == 'None') {
                            error 'SSM command submission failed.'
                        }

                        echo "SSM Command ID: ${commandId}"

                        def status = 'Pending'

                        timeout(time: 3, unit: 'MINUTES') {
                            while (!(status in [
                                'Success',
                                'Failed',
                                'TimedOut',
                                'Cancelled',
                                'Undeliverable',
                                'Terminated'
                            ])) {
                                sleep(time: 5, unit: 'SECONDS')

                                def pollOutput = bat(
                                    returnStdout: true,
                                    script: """@echo off
set AWS_DEFAULT_REGION=%AWS_REGION%
"%AWS_CLI%" ssm get-command-invocation --region %AWS_REGION% --command-id ${commandId} --instance-id %EC2_INSTANCE_ID% --query Status --output text 2>&1 || echo SSM_POLL_COMMAND_FAILED
"""
                                ).trim()

                                if (pollOutput.contains('InvocationDoesNotExist')) {
                                    echo 'SSM invocation is not available yet. Retrying...'
                                } else if (pollOutput.contains('SSM_POLL_COMMAND_FAILED')) {
                                    error "SSM polling command failed: ${pollOutput}"
                                } else {
                                    status = pollOutput
                                }
                            }
                        }

                        if (status != 'Success') {
                            error "SSM connectivity test failed. Status: ${status}"
                        }

                        def output = bat(
                            returnStdout: true,
                            script: """@echo off
set AWS_DEFAULT_REGION=%AWS_REGION%
"%AWS_CLI%" ssm get-command-invocation --region %AWS_REGION% --command-id ${commandId} --instance-id %EC2_INSTANCE_ID% --query StandardOutputContent --output text
"""
                        ).trim()

                        echo "SSM command output: ${output}"

                        if (!output.contains('SSM_CONNECTIVITY_OK')) {
                            error 'Expected SSM output was not found.'
                        }

                        echo 'SSM connectivity test passed.'
                    }
                }
            }
        }

        stage('Deploy to EC2 via SSM') {
            steps {
                withCredentials([
                    usernamePassword(
                        credentialsId: 'aws-ecr-credentials',
                        usernameVariable: 'AWS_ACCESS_KEY_ID',
                        passwordVariable: 'AWS_SECRET_ACCESS_KEY'
                    )
                ]) {
                    script {
                        def deployCommands = [
                            'set -e',
                            'cd /opt/ecommerce_sample_app',
                            'export AWS_DEFAULT_REGION=ap-southeast-2',
                            'aws ecr get-login-password --region ap-southeast-2 | docker login --username AWS --password-stdin 305846488272.dkr.ecr.ap-southeast-2.amazonaws.com',
                            'PREVIOUS_CID=$(docker ps -q --filter label=com.docker.compose.service=user_service --filter status=running)',
                            '[ -n "$PREVIOUS_CID" ] || { echo "Cannot identify previous deployment"; exit 1; }',
                            'PREVIOUS_IMAGE=$(docker inspect --format \'{{.Config.Image}}\' "$PREVIOUS_CID")',
                            'PREVIOUS_TAG=${PREVIOUS_IMAGE##*:}',
                            '[ -n "$PREVIOUS_TAG" ] || { echo "Previous image tag missing"; exit 1; }',
                            'for service in user_service product_service order_service apigateway; do',
                            '  cid=$(docker ps -q --filter label=com.docker.compose.service="$service" --filter status=running)',
                            '  [ -n "$cid" ] || { echo "Previous container missing: $service"; exit 1; }',
                            '  image=$(docker inspect --format \'{{.Config.Image}}\' "$cid")',
                            '  tag=${image##*:}',
                            '  if [ "$tag" != "$PREVIOUS_TAG" ]; then echo "Inconsistent previous tag for $service: $tag"; exit 1; fi',
                            'done',
                            'DEPLOY_OK=0',
                            'rollback_on_exit() {',
                            '  rc=$?',
                            '  if [ "$DEPLOY_OK" != "1" ]; then',
                            '    echo "Deployment failed. Rolling back to $PREVIOUS_TAG"',
                            '    export IMAGE_TAG="$PREVIOUS_TAG"',
                            '    docker compose -f docker-compose.prod.yml pull user_service product_service order_service apigateway && docker compose -f docker-compose.prod.yml up -d --no-deps --force-recreate user_service product_service order_service apigateway || echo "AUTOMATIC ROLLBACK FAILED - MANUAL RECOVERY REQUIRED"',
                            '  fi',
                            '  exit "$rc"',
                            '}',
                            'trap rollback_on_exit EXIT',
                            "export IMAGE_TAG=${env.DEPLOY_IMAGE_TAG}",
                            'docker compose -f docker-compose.prod.yml pull user_service product_service order_service apigateway',
                            'docker compose -f docker-compose.prod.yml up -d --no-deps --force-recreate user_service product_service order_service apigateway',
                            'for service in user_service product_service order_service apigateway; do',
                            '  cid=$(docker compose -f docker-compose.prod.yml ps -q "$service")',
                            '  if [ -z "$cid" ]; then echo "Container missing: $service"; exit 1; fi',
                            '  echo "Checking health: $service"',
                            '  attempt=0',
                            '  health=starting',
                            '  while [ "$attempt" -lt 60 ]; do',
                            '    health=$(docker inspect --format \'{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}\' "$cid")',
                            '    if [ "$health" = "healthy" ]; then break; fi',
                            '    if [ "$health" = "unhealthy" ] || [ "$health" = "no-healthcheck" ]; then echo "$service health failed: $health"; docker logs "$cid" --tail 80; exit 1; fi',
                            '    sleep 5',
                            '    attempt=$((attempt + 1))',
                            '  done',
                            '  if [ "$health" != "healthy" ]; then echo "$service health timeout: $health"; docker logs "$cid" --tail 80; exit 1; fi',
                            '  echo "$service is healthy"',
                            'done',
                            'curl -fsS http://localhost/health',
                            'echo',
                            'docker compose -f docker-compose.prod.yml ps',
                            'DEPLOY_OK=1',
                            'trap - EXIT',
                            'echo EC2_DEPLOYMENT_SUCCESS'
                        ]

                        writeFile(
                            file: 'ssm-deploy-commands.json',
                            text: groovy.json.JsonOutput.toJson([commands: deployCommands])
                        )

                        def commandId = bat(
                            returnStdout: true,
                            script: """@echo off
set AWS_DEFAULT_REGION=%AWS_REGION%
"%AWS_CLI%" ssm send-command --region %AWS_REGION% --instance-ids %EC2_INSTANCE_ID% --document-name AWS-RunShellScript --comment "Ecommerce deployment ${env.DEPLOY_IMAGE_TAG}" --parameters file://ssm-deploy-commands.json --query "Command.CommandId" --output text
"""
                        ).trim()

                        bat 'if exist ssm-deploy-commands.json del ssm-deploy-commands.json'

                        if (!commandId || commandId == 'None') {
                            error 'EC2 deployment command submission failed.'
                        }

                        echo "EC2 deployment command ID: ${commandId}"

                        def status = 'Pending'

                        timeout(time: 30, unit: 'MINUTES') {
                            while (!(status in [
                                'Success', 'Failed', 'TimedOut',
                                'Cancelled', 'Undeliverable', 'Terminated'
                            ])) {
                                sleep(time: 5, unit: 'SECONDS')

                                def pollOutput = bat(
                                    returnStdout: true,
                                    script: """@echo off
set AWS_DEFAULT_REGION=%AWS_REGION%
"%AWS_CLI%" ssm get-command-invocation --region %AWS_REGION% --command-id ${commandId} --instance-id %EC2_INSTANCE_ID% --query Status --output text 2>&1 || echo SSM_POLL_COMMAND_FAILED
"""
                                ).trim()

                                if (pollOutput.contains('InvocationDoesNotExist')) {
                                    echo 'SSM invocation not ready. Retrying...'
                                } else if (pollOutput.contains('SSM_POLL_COMMAND_FAILED')) {
                                    error "SSM polling command failed: ${pollOutput}"
                                } else {
                                    status = pollOutput
                                }
                            }
                        }

                        def output = bat(
                            returnStdout: true,
                            script: """@echo off
set AWS_DEFAULT_REGION=%AWS_REGION%
"%AWS_CLI%" ssm get-command-invocation --region %AWS_REGION% --command-id ${commandId} --instance-id %EC2_INSTANCE_ID% --query StandardOutputContent --output text
"""
                        ).trim()

                        echo "EC2 deployment output: ${output}"

                        if (status != 'Success' || !output.contains('EC2_DEPLOYMENT_SUCCESS')) {
                            error "EC2 deployment failed. SSM status: ${status}"
                        }

                        echo 'EC2 deployment and health verification passed.'
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
            echo 'CI/CD image publishing and EC2 deployment completed successfully.'

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
