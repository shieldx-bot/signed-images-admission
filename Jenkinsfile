pipeline { 
    agent any
    environment { 
        // Lưu ý: TELEGRAM_TOKEN và TELEGRAM_CHAT_ID -> credential kiểu "Secret text"
        TELEGRAM_TOKEN = credentials('telegram-bot-token')
        TELEGRAM_CHAT_ID = credentials('telegram-chat-id')
        DOCKER_CREADS = credentials('docker-hub-login')
        VERSION_IMAGE = 'latest'
        NAME_IMAGE = 'backend_example'
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
                sh  ''' 
                rm -rf backend_example
                git clone https://github.com/shieldx-bot/backend_example.git
              
                '''
                //   cd backend_example
                // docker build -t ${NAME_IMAGE}:${VERSION_IMAGE} .
                // echo "$DOCKER_CREADS_PSW" | docker login -u "$DOCKER_CREADS_USR" --password-stdin
                // docker push ${NAME_IMAGE}:${VERSION_IMAGE}   
            //      sh """
            // msg="Docker Image ${NAME_IMAGE}:${VERSION_IMAGE} has been built and pushed successfully."
            // curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_TOKEN}/sendMessage" \
            // --data-urlencode "chat_id=${TELEGRAM_CHAT_ID}" \
            // --data-urlencode "text=${msg}"
            //         """
            }
        }
        
    }
}

// bạn hãy copy mã này vào file root của thư mục bạn muốn tạo webhook