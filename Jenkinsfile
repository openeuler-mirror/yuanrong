pipeline {
    agent none

    options {
        buildDiscarder(logRotator(numToKeepStr: '30', artifactNumToKeepStr: '10'))
        timeout(time: 2, unit: 'HOURS')
        timestamps()
    }

    environment {
        // 构建镜像
        BUILDER_IMAGE = "${BUILDER_IMAGE ?: 'yuanrong/builder:arm64'}"

        // 构建版本
        BUILD_VERSION = "${BUILD_VERSION ?: 'v0.0.1'}"
        GIT_COMMIT_SHORT = sh(script: "git rev-parse --short=8 HEAD", returnStdout: true).trim()

        // 并发数
        JOBS = "${JOBS ?: '4'}"

        // Go 代理
        GOPROXY = "${GOPROXY ?: 'https://goproxy.cn,direct'}"
    }

    stages {

        stage('Parallel Build') {
            parallel {
                // macOS ARM64 节点构建
                stage('macOS ARM64') {
                    agent {
                        label 'macos-arm64'
                    }
                    steps {
                        script {
                            docker.withServer("tcp://${MACOS_DOCKER_HOST ?: 'localhost:2375'}") {
                                docker.image("${BUILDER_IMAGE}").inside {
                                    sh '''
                                        echo "=== macOS ARM64 构建 ==="
                                        uname -m
                                        echo "构建目标: ${BUILD_TARGET}"
                                        make ${BUILD_TARGET} -j${JOBS}
                                    '''
                                }
                            }
                        }
                    }
                }

                // Linux AMD64 节点构建（可选）
                stage('Linux AMD64') {
                    agent {
                        label 'linux-amd64'
                    }
                    when {
                        expression { params.BUILD_AMD64 == true }
                    }
                    steps {
                        sh '''
                            echo "=== Linux AMD64 构建 ==="
                            uname -m
                            make ${BUILD_TARGET} -j${JOBS}
                        '''
                    }
                }
            }
        }

        stage('Collect Artifacts') {
            agent any
            steps {
                script {
                    // 从 macOS 节点获取产物
                    node('macos-arm64') {
                        sh 'tar -czf output-arm64.tar.gz output/'
                        stash includes: 'output-arm64.tar.gz', name: 'artifacts-arm64'
                    }

                    // 如果有 AMD64 产物也收集
                    node('linux-amd64') {
                        sh 'tar -czf output-amd64.tar.gz output/ || true'
                        stash includes: 'output-amd64.tar.gz', name: 'artifacts-amd64'
                    }
                }
            }
        }

        stage('Package') {
            agent any
            steps {
                unstash 'artifacts-arm64'
                sh '''
                    mkdir -p artifacts
                    tar -xzf output-arm64.tar.gz
                    cp -r output artifacts/arm64
                '''
                script {
                    if (params.BUILD_AMD64) {
                        unstash 'artifacts-amd64'
                        sh '''
                            tar -xzf output-amd64.tar.gz
                            cp -r output artifacts/amd64
                        '''
                    }
                }
                archiveArtifacts artifacts: 'artifacts/**/*', fingerprint: true
            }
        }
    }

    post {
        success {
            echo '✅ 构建成功!'
            echo "产物版本: ${BUILD_VERSION}"
            echo "Git Commit: ${GIT_COMMIT_SHORT}"
        }
        failure {
            echo '❌ 构建失败，请检查日志'
        }
        always {
            cleanWs()
        }
    }
}
