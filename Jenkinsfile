pipeline { 
    agent any
    environment { 
        // Lưu ý: TELEGRAM_TOKEN và TELEGRAM_CHAT_ID -> credential kiểu "Secret text"
        TELEGRAM_TOKEN = credentials('telegram-bot-token')
        TELEGRAM_CHAT_ID = credentials('telegram-chat-id')
        DOCKER_CREADS = credentials('docker-hub-login')
        VERSION_IMAGE = 'v1.0.0'
        NAME_IMAGE = 'backend_example'
        USERNAME = "shieldxbot"
      
    }

    options {
        timestamps()
    }

    // Requires Jenkins GitHub plugin + a GitHub webhook pointed to: https://<jenkins>/github-webhook/
    triggers {
        githubPush()
    }

    stages {
        stage('Build  & Push Docker Image') {
            steps {
            withCredentials([usernamePassword(credentialsId: 'docker-hub-login', usernameVariable: 'DOCKER_HUB_USR', passwordVariable: 'DOCKER_HUB_PSW')]) { 
                  sh  ''' 
                rm -rf backend_example
                git clone https://github.com/shieldx-bot/backend_example.git
                cd backend_example
                docker build -t ${USERNAME}/${NAME_IMAGE}:${VERSION_IMAGE} .
                echo "$DOCKER_HUB_PSW" | docker login -u "$DOCKER_HUB_USR" --password-stdin
                docker push  ${USERNAME}/${NAME_IMAGE}:${VERSION_IMAGE}   
                echo " success build and push image "
                '''
                 script { 
                     def digest = sh (
                    script: "docker inspect --format='{{index .RepoDigests 0}}' ${USERNAME}/${NAME_IMAGE}:${VERSION_IMAGE}"
                    , returnStdout: true
                ).trim()
                env.IMAGE_DIGEST = digest
                echo "IMAGE_DIGEST: ${env.IMAGE_DIGEST}"
                sh 'echo "Will deploy image digest: $IMAGE_DIGEST"'

                 }
                 withCredentials([
                     file(credentialsId: 'cosign-key', variable: 'COSIGN_KEY_FILE'),
                     string(credentialsId: 'cosign-passphrase', variable: 'COSIGN_PASSWORD')
                     ]) {
                         sh '''
                       
                           set -eu
    export COSIGN_PASSWORD="$COSIGN_PASSWORD"
    cosign sign --yes --key "$COSIGN_KEY_FILE" "$IMAGE_DIGEST"
    cosign tree "$IMAGE_DIGEST" 
                        '''
                         }
            } 
            }
        }
        
    }
}