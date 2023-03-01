package troubleshoot

import (
	"github.com/Dynatrace/dynatrace-operator/src/dtclient"
	"github.com/Dynatrace/dynatrace-operator/src/kubeobjects"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/net/http/httpproxy"
	"k8s.io/apimachinery/pkg/types"
)

func checkProxySettings(troubleshootCtx *troubleshootContext) error {
	return checkProxySettingsWithLog(troubleshootCtx, troubleshootCtx.baseLog.WithName("proxy"))
}

func checkProxySettingsWithLog(troubleshootCtx *troubleshootContext, log logr.Logger) error {

	var proxyURL string
	logNewCheckf(log, "Analyzing proxy settings ...")

	proxySettingsAvailable := false
	if troubleshootCtx.dynakube.HasProxy() {
		proxySettingsAvailable = true
		logInfof(log, "Reminder: Proxy settings in the Dynakube do not apply to pulling of pod images. Please set your proxy on accordingly on node level.")
		logWarningf(log, "Proxy settings in the Dynakube are ignored for codeModules images due to technical limitations.")

		var err error
		proxyURL, err = getProxyURL(log, troubleshootCtx)
		if err != nil {
			logErrorf(log, "Unexpected error when reading proxy settings from Dynakube: %v", err)
			return nil
		}
	}

	if checkEnvironmentProxySettings(log, proxyURL) {
		proxySettingsAvailable = true
	}

	if !proxySettingsAvailable {
		logOkf(log, "No proxy settings found.")
	}
	return nil
}

func checkEnvironmentProxySettings(log logr.Logger, proxyURL string) bool {
	envProxy := getEnvProxySettings()
	if envProxy == nil {
		return false
	}

	logInfof(log, "Searching environment for proxy settings ...")
	if envProxy.HTTPProxy != "" {
		logWarningf(log, "HTTP_PROXY is set in environment. This setting will be used by the operator for codeModule image pulls.")
		if proxySettingsDiffer(envProxy.HTTPProxy, proxyURL) {
			logWarningf(log, "Proxy settings in the Dynakube and HTTP_PROXY differ.")
		}
	}
	if envProxy.HTTPSProxy != "" {
		logWarningf(log, "HTTPS_PROXY is set in environment. This setting will be used by the operator for codeModule image pulls.")
		if proxySettingsDiffer(envProxy.HTTPSProxy, proxyURL) {
			logWarningf(log, "Proxy settings in the Dynakube and HTTPS_PROXY differ.")
		}
	}
	return true
}

func proxySettingsDiffer(envProxy, dynakubeProxy string) bool {
	return envProxy != "" && dynakubeProxy != "" && envProxy != dynakubeProxy
}

func getEnvProxySettings() *httpproxy.Config {
	envProxy := httpproxy.FromEnvironment()
	if envProxy.HTTPProxy != "" || envProxy.HTTPSProxy != "" {
		return envProxy
	}
	return nil
}

func applyProxySettings(log logr.Logger, troubleshootCtx *troubleshootContext) error {
	proxyURL, err := getProxyURL(log, troubleshootCtx)
	if err != nil {
		return err
	}

	if proxyURL != "" {
		err := troubleshootCtx.SetTransportProxy(log, proxyURL)
		if err != nil {
			return errors.Wrapf(err, "error parsing proxy value")
		}
	}

	return nil
}

func getProxyURL(log logr.Logger, troubleshootCtx *troubleshootContext) (string, error) {
	if troubleshootCtx.dynakube.Spec.Proxy == nil {
		return "", nil
	}

	if troubleshootCtx.dynakube.Spec.Proxy.Value != "" {
		return troubleshootCtx.dynakube.Spec.Proxy.Value, nil
	}

	if troubleshootCtx.dynakube.Spec.Proxy.ValueFrom != "" {
		err := setProxySecret(log, troubleshootCtx)
		if err != nil {
			return "", err
		}

		proxyUrl, err := kubeobjects.ExtractToken(troubleshootCtx.proxySecret, dtclient.CustomProxySecretKey)
		if err != nil {
			return "", errors.Wrapf(err, "failed to extract proxy secret field")
		}
		return proxyUrl, nil
	}
	return "", nil
}

func setProxySecret(log logr.Logger, troubleshootCtx *troubleshootContext) error {
	if troubleshootCtx.proxySecret != nil {
		return nil
	}

	query := kubeobjects.NewSecretQuery(troubleshootCtx.context, nil, troubleshootCtx.apiReader, log)
	secret, err := query.Get(types.NamespacedName{
		Namespace: troubleshootCtx.namespaceName,
		Name:      troubleshootCtx.dynakube.Spec.Proxy.ValueFrom})

	if err != nil {
		return errors.Wrapf(err, "'%s:%s' proxy secret is missing",
			troubleshootCtx.namespaceName, troubleshootCtx.dynakube.Spec.Proxy.ValueFrom)
	}

	troubleshootCtx.proxySecret = &secret
	logInfof(log, "proxy secret '%s:%s' exists",
		troubleshootCtx.namespaceName, troubleshootCtx.dynakube.Spec.Proxy.ValueFrom)
	return nil
}
