package support_archive

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	dynatracev1beta1 "github.com/Dynatrace/dynatrace-operator/src/api/v1beta1"
	"github.com/Dynatrace/dynatrace-operator/src/scheme/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestTroubleshootCollector(t *testing.T) {
	logBuffer := bytes.Buffer{}
	log := newSupportArchiveLogger(&logBuffer)

	clt := fake.NewClientWithIndex(
		&appsv1.Deployment{
			TypeMeta:   typeMeta("Deployment"),
			ObjectMeta: objectMeta("deployment1"),
		},
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "core/v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "random",
			},
		},
		&dynatracev1beta1.DynaKube{
			TypeMeta:   typeMeta("DynaKube"),
			ObjectMeta: objectMeta("dynakube1"),
		},
	)

	tarBuffer := bytes.Buffer{}
	supportArchive := tarball{
		tarWriter: tar.NewWriter(&tarBuffer),
	}

	ctx := context.TODO()
	require.NoError(t, newTroubleshootCollector(ctx, log, supportArchive, testOperatorNamespace, clt, rest.Config{}).Do())

	tarReader := tar.NewReader(&tarBuffer)

	hdr, err := tarReader.Next()
	require.NoError(t, err)
	assert.Equal(t, TroublshootOutputFileName, hdr.Name)

	troubleshootFile := make([]byte, hdr.Size)
	bytesRead, err := tarReader.Read(troubleshootFile)
	if !errors.Is(err, io.EOF) {
		require.NoError(t, err)
	}
	assert.Equal(t, hdr.Size, int64(bytesRead))
	_, err = tarReader.Next()
	require.ErrorIs(t, err, io.EOF)
}
