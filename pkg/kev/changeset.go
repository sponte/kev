/**
 * Copyright 2020 Appvia Ltd <info@appvia.io>
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

package kev

import (
	"fmt"
	"reflect"

	"github.com/appvia/kev/pkg/kev/config"
	"github.com/appvia/kev/pkg/kev/log"
)

const (
	CREATE = "create"
	UPDATE = "update"
	DELETE = "delete"
)

// changes returns a flat list of all available changes
func (cset changeset) changes() []change {
	var out []change
	if !reflect.DeepEqual(cset.version, change{}) {
		out = append(out, cset.version)
	}
	out = append(out, cset.services...)
	out = append(out, cset.volumes...)
	return out
}

// HasNoPatches informs if a changeset has any patches to apply.
func (cset changeset) HasNoPatches() bool {
	return len(cset.changes()) <= 0
}

func (cset changeset) applyVersionPatchesIfAny(o *composeOverride) string {
	chg := cset.version
	if reflect.DeepEqual(chg, change{}) {
		return ""
	}
	return chg.patchVersion(o)
}

func (cset changeset) applyServicesPatchesIfAny(o *composeOverride) []string {
	var out []string
	for _, change := range cset.services {
		out = append(out, change.patchService(o))
	}
	return out
}

func (cset changeset) applyVolumesPatchesIfAny(o *composeOverride) []string {
	var out []string
	for _, change := range cset.volumes {
		out = append(out, change.patchVolume(o))
	}
	return out
}

func (chg change) patchVersion(override *composeOverride) string {
	if chg.Type != UPDATE {
		return ""
	}
	pre := override.Version
	newValue := chg.Value.(string)
	override.Version = newValue

	msg := fmt.Sprintf("version %s updated to %s", pre, newValue)
	log.Debugf(msg)
	return msg
}

func (chg change) patchService(override *composeOverride) string {
	switch chg.Type {
	case CREATE:
		newValue := chg.Value.(ServiceConfig).condenseLabels(config.BaseServiceLabels)
		override.Services = append(override.Services, newValue)
		msg := fmt.Sprintf("added service: %s", newValue.Name)
		log.Debugf(msg)
		return msg
	case DELETE:
		switch {
		case chg.Parent == "environment":
			delete(override.Services[chg.Index.(int)].Environment, chg.Target)
			msg := fmt.Sprintf("removed env var: %s from service %s", chg.Target, override.Services[chg.Index.(int)].Name)
			log.Debugf(msg)
			return msg
		default:
			deletedSvcName := override.Services[chg.Index.(int)].Name
			override.Services = append(override.Services[:chg.Index.(int)], override.Services[chg.Index.(int)+1:]...)
			msg := fmt.Sprintf("removed service: %s", deletedSvcName)
			log.Debugf(msg)
			return msg
		}
	case UPDATE:
		switch chg.Parent {
		case "labels":
			pre, canUpdate := override.Services[chg.Index.(int)].Labels[chg.Target]
			newValue := chg.Value.(string)
			override.Services[chg.Index.(int)].Labels[chg.Target] = newValue
			if canUpdate {
				log.Debugf("service [%s], label [%s] updated, from:[%s] to:[%s]", override.Services[chg.Index.(int)].Name, chg.Target, pre, newValue)
			}
		case "extensions":
			svc := override.Services[chg.Index.(int)]
			svcName := svc.Name

			newValue, ok := chg.Value.(map[string]interface{})
			if !ok {
				log.Debugf("unable to update service [%s], invalid value %+v", svcName, newValue)
				return ""
			}

			if svc.Extensions == nil {
				svc.Extensions = make(map[string]interface{})
			}

			svc.Extensions[config.K8SExtensionKey] = newValue
			log.Debugf("service [%s] extensions updated to %+v", svcName, newValue)
		}
	}
	return ""
}

func (chg change) patchVolume(override *composeOverride) string {
	switch chg.Type {
	case CREATE:
		newValue := chg.Value.(VolumeConfig).condenseLabels(config.BaseVolumeLabels)
		override.Volumes[chg.Index.(string)] = newValue
		msg := fmt.Sprintf("added volume: %s", chg.Index.(string))
		log.Debugf(msg)
		return msg
	case DELETE:
		delete(override.Volumes, chg.Index.(string))
		msg := fmt.Sprintf("removed volume: %s", chg.Index.(string))
		log.Debugf(msg)
		return msg
	}
	return ""
}
