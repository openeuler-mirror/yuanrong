package instancepool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	commonTypes "yuanrong/pkg/common/faas_common/types"
	"yuanrong/pkg/functionscaler/types"
)

func TestConfigVolume(t *testing.T) {
	serviceName := "testService"
	microServiceName := "testMicroService"
	ss := stsSecret{
		param: &types.FunctionSpecification{
			StsMetaData: commonTypes.StsMetaData{
				ServiceName:  serviceName,
				MicroService: microServiceName,
			},
		},
		req: types.PodRequest{
			FunSvcID: "testFunc",
		},
	}
	vb := volumeBuilder{
		volumes: make([]corev1.Volume, 0),
		mounts:  make(map[container][]corev1.VolumeMount),
	}
	ss.configVolume(&vb)
	foundBits := 0
	for _, v := range vb.volumes {
		if v.Name == volumeStsConfig && v.VolumeSource.Secret.SecretName == "testFunc" {
			foundBits |= 0b0001
		}
		if v.Name == volumeCertVolume {
			foundBits |= 0b0010
		}
	}
	assert.Equal(t, 0b0011, foundBits)
	foundBits = 0
	for _, vm := range vb.mounts[containerRuntimeManager] {
		if vm.Name == volumeCertVolume && vm.MountPath == fmt.Sprintf("%s/%s/%s/", baseCertFilePath, serviceName,
			microServiceName) {
			foundBits |= 0b0000001
		}
		if vm.Name == volumeStsConfig && vm.MountPath == fmt.Sprintf("%s/%s/%s/%s/apple/a", baseCertFilePath,
			serviceName, microServiceName, microServiceName) {
			foundBits |= 0b0000010
		}
		if vm.Name == volumeStsConfig && vm.MountPath == fmt.Sprintf("%s/%s/%s/%s/boy/b", baseCertFilePath,
			serviceName, microServiceName, microServiceName) {
			foundBits |= 0b0000100
		}
		if vm.Name == volumeStsConfig && vm.MountPath == fmt.Sprintf("%s/%s/%s/%s/cat/c", baseCertFilePath,
			serviceName, microServiceName, microServiceName) {
			foundBits |= 0b0001000
		}
		if vm.Name == volumeStsConfig && vm.MountPath == fmt.Sprintf("%s/%s/%s/%s/dog/d", baseCertFilePath,
			serviceName, microServiceName, microServiceName) {
			foundBits |= 0b0010000
		}
		if vm.Name == volumeStsConfig && vm.MountPath == fmt.Sprintf("%s/%s/%s/%s.ini", baseCertFilePath,
			serviceName, microServiceName, microServiceName) {
			foundBits |= 0b0100000
		}
		if vm.Name == volumeStsConfig && vm.MountPath == fmt.Sprintf("%s/%s/%s/%s.sts.p12", baseCertFilePath,
			serviceName, microServiceName, microServiceName) {
			foundBits |= 0b1000000
		}
	}
	assert.Equal(t, 0b1111111, foundBits)
}
