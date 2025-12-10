// get data out of config //
package config

import (
	glb "github.ibmgcloud.net/dth/inbound_parser/global_structs"
)

// look through the Emails field in the servicedesks to find the addressee
func GetServiceDeskFromMail(cfg *glb.Config, emailTo string) *glb.ServiceDesk {
	for _, jiraInstall := range cfg.JiraInstalls {
		for _, srd := range jiraInstall.ServiceDesks {
			for _, srdEmail := range srd.Emails {
				if srdEmail == emailTo {
					return srd
				}
			}
		}
	}
	return nil
}

func GetServiceDeskFromId(cfg *glb.Config, id string) *glb.ServiceDesk {
	for _, jiraInstall := range cfg.JiraInstalls {
		for _, srd := range jiraInstall.ServiceDesks {
			if srd.Id == id && !srd.OnlyCreateEventRequests {
				return srd
			}
		}
	}
	return nil
}

// look through the Emails field in the jira install to find the addressee
func GetJiraInstallFromMail(cfg *glb.Config, emailTo string) *glb.JiraInstall {
	for _, jiraInstall := range cfg.JiraInstalls {
		for _, jiraInstallEmail := range jiraInstall.Emails {
			if jiraInstallEmail == emailTo {
				return jiraInstall
			}
		}
	}
	return nil
}
