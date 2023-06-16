/*
 * Copyright 2023 Damian Peckett <damian@pecke.tt>.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReconcilerScriptSpec struct {
	// Name is the name of the script.
	Name string `json:"name"`
	// Encoded is a base64 encoded string of the script. We use
	// base64 encoding here to prevent issues with ytt markers in
	// the script getting prematurely evaluated.
	Encoded string `json:"encoded"`
}

// ReconcilerSpec defines the desired state of Reconciler
type ReconcilerSpec struct {
	// For is a list of resource GVKs to reconcile.
	For []metav1.TypeMeta `json:"for,omitempty"`
	// Scripts is a list of scripts to execute for this reconciler.
	Scripts []ReconcilerScriptSpec `json:"scripts,omitempty"`
}

// ReconcilerStatus defines the observed state of Reconciler
type ReconcilerStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Reconciler is the Schema for the reconcilers API
type Reconciler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReconcilerSpec   `json:"spec,omitempty"`
	Status ReconcilerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReconcilerList contains a list of Reconciler
type ReconcilerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Reconciler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Reconciler{}, &ReconcilerList{})
}
