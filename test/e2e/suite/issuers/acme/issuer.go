/*
Copyright 2020 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package acme

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	"github.com/jetstack/cert-manager/test/e2e/framework"
	"github.com/jetstack/cert-manager/test/e2e/util"
	"github.com/jetstack/cert-manager/test/unit/gen"
)

const invalidACMEURL = "http://not-a-real-acme-url.com"
const testingACMEEmail = "test@example.com"
const testingACMEEmailAlternative = "another-test@example.com"
const testingACMEPrivateKey = "test-acme-private-key"

var _ = framework.CertManagerDescribe("ACME Issuer", func() {
	f := framework.NewDefaultFramework("create-acme-issuer")

	issuerName := "test-acme-issuer"

	AfterEach(func() {
		By("Cleaning up")
		f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Delete(context.TODO(), issuerName, metav1.DeleteOptions{})
		f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Delete(context.TODO(), testingACMEPrivateKey, metav1.DeleteOptions{})
	})

	It("should register ACME account", func() {
		acmeIssuer := gen.Issuer(issuerName,
			gen.SetIssuerNamespace(f.Namespace.Name),
			gen.SetIssuerACMEEmail(testingACMEEmail),
			gen.SetIssuerACMEURL(f.Config.Addons.ACMEServer.URL),
			gen.SetIssuerACMESkipTLSVerify(true),
			gen.SetIssuerACMEPrivKeyRef(testingACMEPrivateKey))
		By("Creating an Issuer")
		_, err := f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Create(context.TODO(), acmeIssuer, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Issuer to become Ready")
		err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			v1.IssuerCondition{
				Type:   v1.IssuerConditionReady,
				Status: cmmeta.ConditionTrue,
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the ACME account URI is set")
		err = util.WaitForIssuerStatusFunc(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			func(i *v1.Issuer) (bool, error) {
				if i.GetStatus().ACMEStatus().URI == "" {
					return false, nil
				}
				return true, nil
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying ACME account private key exists")
		secret, err := f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Get(context.TODO(), testingACMEPrivateKey, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		if len(secret.Data) != 1 {
			Fail("Expected 1 key in ACME account private key secret, but there was %d", len(secret.Data))
		}
	})

	It("should recover a lost ACME account URI", func() {
		acmeIssuer := gen.Issuer(issuerName,
			gen.SetIssuerNamespace(f.Namespace.Name),
			gen.SetIssuerACMEEmail(testingACMEEmail),
			gen.SetIssuerACMEURL(f.Config.Addons.ACMEServer.URL),
			gen.SetIssuerACMESkipTLSVerify(true),
			gen.SetIssuerACMEPrivKeyRef(testingACMEPrivateKey))
		By("Creating an Issuer")
		_, err := f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Create(context.TODO(), acmeIssuer, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Issuer to become Ready")
		err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			v1.IssuerCondition{
				Type:   v1.IssuerConditionReady,
				Status: cmmeta.ConditionTrue,
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the ACME account URI is set")
		var finalURI string
		err = util.WaitForIssuerStatusFunc(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			func(i *v1.Issuer) (bool, error) {
				if i.GetStatus().ACMEStatus().URI == "" {
					return false, nil
				}
				finalURI = i.GetStatus().ACMEStatus().URI
				return true, nil
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying ACME account private key exists")
		secret, err := f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Get(context.TODO(), testingACMEPrivateKey, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		if len(secret.Data) != 1 {
			Fail("Expected 1 key in ACME account private key secret, but there was %d", len(secret.Data))
		}

		By("Deleting the Issuer")
		err = f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Delete(context.TODO(), acmeIssuer.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Recreating the Issuer")
		acmeIssuer = gen.Issuer(issuerName,
			gen.SetIssuerNamespace(f.Namespace.Name),
			gen.SetIssuerACMEEmail(testingACMEEmail),
			gen.SetIssuerACMEURL(f.Config.Addons.ACMEServer.URL),
			gen.SetIssuerACMESkipTLSVerify(true),
			gen.SetIssuerACMEPrivKeyRef(testingACMEPrivateKey))
		_, err = f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Create(context.TODO(), acmeIssuer, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Issuer to become Ready")
		err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			v1.IssuerCondition{
				Type:   v1.IssuerConditionReady,
				Status: cmmeta.ConditionTrue,
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the ACME account URI has been recovered correctly")
		err = util.WaitForIssuerStatusFunc(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			func(i *v1.Issuer) (bool, error) {
				uri := i.GetStatus().ACMEStatus().URI
				if uri == "" {
					return false, nil
				}
				if uri != finalURI {
					return false, fmt.Errorf("expected account URI to equal %q, but was %q", finalURI, uri)
				}
				return true, nil
			})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should fail to register an ACME account", func() {
		acmeIssuer := gen.Issuer(issuerName,
			gen.SetIssuerNamespace(f.Namespace.Name),
			gen.SetIssuerACMEEmail(testingACMEEmail),
			gen.SetIssuerACMEURL(invalidACMEURL),
			gen.SetIssuerACMESkipTLSVerify(true),
			gen.SetIssuerACMEPrivKeyRef(testingACMEPrivateKey))

		By("Creating an Issuer with an invalid server")
		_, err := f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Create(context.TODO(), acmeIssuer, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Issuer to become non-Ready")
		err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			v1.IssuerCondition{
				Type:   v1.IssuerConditionReady,
				Status: cmmeta.ConditionFalse,
			})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should handle updates to the email field", func() {
		acmeIssuer := gen.Issuer(issuerName,
			gen.SetIssuerNamespace(f.Namespace.Name),
			gen.SetIssuerACMEEmail(testingACMEEmail),
			gen.SetIssuerACMEURL(f.Config.Addons.ACMEServer.URL),
			gen.SetIssuerACMESkipTLSVerify(true),
			gen.SetIssuerACMEPrivKeyRef(testingACMEPrivateKey))

		By("Creating an Issuer")
		acmeIssuer, err := f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Create(context.TODO(), acmeIssuer, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Issuer to become Ready")
		err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			v1.IssuerCondition{
				Type:   v1.IssuerConditionReady,
				Status: cmmeta.ConditionTrue,
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the ACME account URI is set")
		err = util.WaitForIssuerStatusFunc(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			func(i *v1.Issuer) (bool, error) {
				if i.GetStatus().ACMEStatus().URI == "" {
					return false, nil
				}
				return true, nil
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying ACME account private key exists")
		secret, err := f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Get(context.TODO(), testingACMEPrivateKey, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		if len(secret.Data) != 1 {
			Fail("Expected 1 key in ACME account private key secret, but there was %d", len(secret.Data))
		}

		By("Verifying the ACME account email has been registered")
		err = util.WaitForIssuerStatusFunc(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			func(i *v1.Issuer) (bool, error) {
				registeredEmail := i.GetStatus().ACMEStatus().LastRegisteredEmail
				if registeredEmail == testingACMEEmail {
					return true, nil
				}
				return false, nil
			})
		Expect(err).NotTo(HaveOccurred())

		By("Changing the email field")
		acmeIssuer, err = f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Get(context.TODO(), acmeIssuer.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		acmeIssuer.Spec.ACME.Email = testingACMEEmailAlternative
		acmeIssuer, err = f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name).Update(context.TODO(), acmeIssuer, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Issuer to become Ready")
		err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			v1.IssuerCondition{
				Type:   v1.IssuerConditionReady,
				Status: cmmeta.ConditionTrue,
			})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the changed ACME account email has been registered")
		err = util.WaitForIssuerStatusFunc(f.CertManagerClientSet.CertmanagerV1().Issuers(f.Namespace.Name),
			acmeIssuer.Name,
			func(i *v1.Issuer) (bool, error) {
				registeredEmail := i.GetStatus().ACMEStatus().LastRegisteredEmail
				if registeredEmail == testingACMEEmailAlternative {
					return true, nil
				}
				return false, nil
			})
		Expect(err).NotTo(HaveOccurred())
	})
})
