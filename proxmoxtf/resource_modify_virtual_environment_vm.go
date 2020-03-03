/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package proxmoxtf

import (
	"fmt"
	"log"

	"github.com/danitso/terraform-provider-proxmox/proxmox"
	"github.com/hashicorp/terraform/helper/schema"
)

const (
	dvResourceModifyVirtualEnvironmentVMNewDiskSize = "0G"
	dvResourceModifyVirtualEnvironmentVMReboot      = true
	dvResourceVirtualEnvironmentVMModifyDiskName    = "virtio"
	dvResourceVirtualEnvironmentVMModifyDiskIndex   = 0
	dvResourceVirtualEnvironmentVMModifyMoveDelete  = false
	dvResourceVirtualEnvironmentVMModifyStorage     = ""

	mkResourceModifyVirtualEnvironmentVMNewDiskSize     = "new_disksize"
	mkResourceModifyVirtualEnvironmentVMCurrentDiskSize = "current_disksize"
	mkResourceModifyVirtualEnvironmentVMReboot          = "reboot"
	mkResourceModifyVirtualEnvironmentVMModifyCluster   = "modify_cluster"
	mkResourceVirtualEnvironmentVMModifyClusterNodeName = "node_name"
	mkResourceVirtualEnvironmentVMModifyClusterVMs      = "vms"
	mkResourceVirtualEnvironmentVMModifyClusterVMId     = "vm_id"
	mkResourceVirtualEnvironmentVMModifyDiskName        = "disk_name"
	mkResourceVirtualEnvironmentVMModifyDiskIndex       = "disk_index"
	mkResourceVirtualEnvironmentVMModifyMoveDisk        = "move_disk"
	mkResourceVirtualEnvironmentVMModifyStorage         = "target_storage"
	mkResourceVirtualEnvironmentVMModifyDelete          = "delete"
)

func resourceModifyVirtualEnvironmentVM() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			mkResourceModifyVirtualEnvironmentVMReboot: {
				Type:        schema.TypeBool,
				Description: "Trigger to reboot the Virtual Machine",
				Optional:    true,
				Default:     dvResourceModifyVirtualEnvironmentVMReboot,
			},
			mkResourceModifyVirtualEnvironmentVMModifyCluster: {
				Type:        schema.TypeList,
				Description: "List with cluster information for vm modification",
				Optional:    false,
				Required:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						mkResourceVirtualEnvironmentVMModifyClusterNodeName: {
							Type:        schema.TypeString,
							Description: "The Node Name containing the vm ids to modify",
							Optional:    false,
							Required:    true,
						},
						mkResourceModifyVirtualEnvironmentVMNewDiskSize: {
							Type:        schema.TypeString,
							Description: "New size of the disk",
							Optional:    true,
							Default:     dvResourceModifyVirtualEnvironmentVMNewDiskSize,
						},
						mkResourceVirtualEnvironmentVMModifyClusterVMId: {
							Type:        schema.TypeInt,
							Description: "VmId to modify",
							Optional:    false,
							Required:    true,
						},
						mkResourceVirtualEnvironmentVMModifyDiskName: {
							Type:        schema.TypeString,
							Description: "Name of the disk to modify",
							Optional:    true,
							Default:     dvResourceVirtualEnvironmentVMModifyDiskName,
						},
						mkResourceModifyVirtualEnvironmentVMCurrentDiskSize: {
							Type:        schema.TypeString,
							Description: "computed size of disk",
							Computed:    true,
						},
						mkResourceVirtualEnvironmentVMModifyDiskIndex: {
							Type:        schema.TypeInt,
							Description: "Index for disk to modify",
							Optional:    true,
							Default:     dvResourceVirtualEnvironmentVMModifyDiskIndex,
						},
						mkResourceVirtualEnvironmentVMModifyStorage: {
							Type:        schema.TypeString,
							Description: "Target Storage",
							Optional:    true,
							Default:     dvResourceVirtualEnvironmentVMModifyStorage,
						},
						mkResourceVirtualEnvironmentVMModifyDelete: {
							Type:        schema.TypeBool,
							Description: "Bool if old disk shall be deleted or kept",
							Optional:    true,
							Default:     dvResourceVirtualEnvironmentVMModifyMoveDelete,
						},
					},
				},
			},
		},
		Create: resourceModifyVirtualEnvironmentVMCreate,
		Read:   resourceModifyVirtualEnvironmentVMRead,
		Update: resourceModifyVirtualEnvironmentVMUpdate,
		Delete: resourceModifyVirtualEnvironmentVMDelete,
	}
}

func resourceModifyVirtualEnvironmentVMCreate(d *schema.ResourceData, m interface{}) error {
	reboot := d.Get(mkResourceModifyVirtualEnvironmentVMReboot).(bool)
	nodeInfo := d.Get(mkResourceModifyVirtualEnvironmentVMModifyCluster).([]interface{})

	config := m.(providerConfiguration)
	veClient, err := config.GetVEClient()
	nodeBlock := nodeInfo[0].(map[string]interface{})

	if err != nil {
		return err
	}

	nodeName := nodeBlock[mkResourceVirtualEnvironmentVMModifyClusterNodeName].(string)
	vmID := nodeBlock[mkResourceVirtualEnvironmentVMModifyClusterVMId].(int)
	diskSize := nodeBlock[mkResourceModifyVirtualEnvironmentVMNewDiskSize].(string)
	diskName := nodeBlock[mkResourceVirtualEnvironmentVMModifyDiskName].(string)
	diskIndex := nodeBlock[mkResourceVirtualEnvironmentVMModifyDiskIndex].(int)
	targetStorage := nodeBlock[mkResourceVirtualEnvironmentVMModifyStorage].(string)
	delete := proxmox.CustomBool(nodeBlock[mkResourceVirtualEnvironmentVMModifyDelete].(bool))

	diskFullName := fmt.Sprintf("%s%d", diskName, diskIndex)
	rebootTimeout := 300

	if targetStorage != dvResourceVirtualEnvironmentVMModifyStorage {
		log.Printf("[DEBUG] Move Disk: node=%s vm=%d storage=%s diskName=%s delete=%t", nodeName, vmID, targetStorage, diskFullName, delete)
		response, err := veClient.MoveDisk(nodeName, vmID, &proxmox.VirtualEnvironmentVMMoveDiskRequestBody{
			Disk:    &diskFullName,
			Storage: &targetStorage,
			Delete:  &delete,
		})

		if err != nil {
			return err
		}

		log.Println("[DEBUG] Response data move disk:", *response)
		err = veClient.WaitForTask(nodeName, *response)

		if err != nil {
			return err
		}
	}

	if diskSize != dvResourceModifyVirtualEnvironmentVMNewDiskSize {
		log.Printf("[DEBUG] Resize Disk: node=%s vm=%d diskSize=%s diskName=%s", nodeName, vmID, diskSize, diskFullName)
		err = veClient.ResizeDisk(nodeName, vmID, &proxmox.VirtualEnvironmentVMResizeDiskRequestBody{
			Disk: &diskFullName,
			Size: &diskSize,
		})

		if err != nil {
			return err
		}
	}

	if reboot {
		log.Printf("[DEBUG] Reboot Resize: node=%s vm=%d ", nodeName, vmID)
		response, err := veClient.RebootVM(nodeName, vmID, &proxmox.VirtualEnvironmentVMRebootRequestBody{
			Timeout: &rebootTimeout,
		})

		if err != nil {
			return err
		}

		log.Println("[DEBUG] Response Data shutdown:", *response)
		err = veClient.WaitForTask(nodeName, *response)
	}

	d.SetId(fmt.Sprintf("%s%d", nodeName, vmID))
	return resourceModifyVirtualEnvironmentVMRead(d, m)
}

func resourceModifyVirtualEnvironmentVMRead(d *schema.ResourceData, m interface{}) error {
	nodeInfo := d.Get(mkResourceModifyVirtualEnvironmentVMModifyCluster).([]interface{})

	config := m.(providerConfiguration)
	veClient, err := config.GetVEClient()
	nodeBlock := nodeInfo[0].(map[string]interface{})

	if err != nil {
		return err
	}

	nodeName := nodeBlock[mkResourceVirtualEnvironmentVMModifyClusterNodeName].(string)
	vmID := nodeBlock[mkResourceVirtualEnvironmentVMModifyClusterVMId].(int)
	diskIndex := nodeBlock[mkResourceVirtualEnvironmentVMModifyDiskIndex].(int)
	diskName := nodeBlock[mkResourceVirtualEnvironmentVMModifyDiskName].(string)
	vmConfig, err := veClient.GetVM(nodeName, vmID)

	if err != nil {
		return err
	}

	var size string
	switch diskName {
	case "virtio":
		size = readVirtioDevice(diskIndex, vmConfig, d)
	case "sata":
		size = readSATADevice(diskIndex, vmConfig, d)
	case "scsi":
		size = readSCSIDevice(diskIndex, vmConfig, d)
	case "ide":
		size = readIDEDevice(diskIndex, vmConfig, d)
	}

	nodeBlock[mkResourceModifyVirtualEnvironmentVMCurrentDiskSize] = size
	nodeInfo[0] = nodeBlock
	d.Set(mkResourceModifyVirtualEnvironmentVMModifyCluster, nodeInfo)

	return nil
}

func resourceModifyVirtualEnvironmentVMUpdate(d *schema.ResourceData, m interface{}) error {
	if d.HasChange(mkResourceModifyVirtualEnvironmentVMModifyCluster) {
		resourceModifyVirtualEnvironmentVMCreate(d, m)
	}

	return resourceModifyVirtualEnvironmentVMRead(d, m)
}

func resourceModifyVirtualEnvironmentVMDelete(d *schema.ResourceData, m interface{}) error {
	return nil
}

func readVirtioDevice(index int, data *proxmox.VirtualEnvironmentVMGetResponseData, d *schema.ResourceData) string {
	storageDevices := []*proxmox.CustomVirtualIODeviceConfig{
		data.VirtualIODevice0,
		data.VirtualIODevice1,
		data.VirtualIODevice2,
		data.VirtualIODevice3,
		data.VirtualIODevice4,
		data.VirtualIODevice5,
		data.VirtualIODevice6,
		data.VirtualIODevice7,
		data.VirtualIODevice8,
		data.VirtualIODevice9,
		data.VirtualIODevice10,
		data.VirtualIODevice11,
		data.VirtualIODevice12,
		data.VirtualIODevice13,
		data.VirtualIODevice14,
		data.VirtualIODevice15,
	}

	log.Printf("[DEBUG] current size is %s", *storageDevices[index].Size)
	return *storageDevices[index].Size
}

func readSATADevice(index int, data *proxmox.VirtualEnvironmentVMGetResponseData, d *schema.ResourceData) string {
	storageDevices := []*proxmox.CustomStorageDevice{
		data.SATADevice0,
		data.SATADevice1,
		data.SATADevice2,
		data.SATADevice3,
		data.SATADevice4,
		data.SATADevice5,
	}

	log.Printf("[DEBUG] current size is %s", *storageDevices[index].Size)
	return *storageDevices[index].Size
}

func readIDEDevice(index int, data *proxmox.VirtualEnvironmentVMGetResponseData, d *schema.ResourceData) string {
	storageDevices := []*proxmox.CustomStorageDevice{
		data.IDEDevice0,
		data.IDEDevice1,
		data.IDEDevice2,
	}

	log.Printf("[DEBUG] current size is %s", *storageDevices[index].Size)
	return *storageDevices[index].Size
}

func readSCSIDevice(index int, data *proxmox.VirtualEnvironmentVMGetResponseData, d *schema.ResourceData) string {
	storageDevices := []*proxmox.CustomStorageDevice{
		data.SCSIDevice0,
		data.SCSIDevice1,
		data.SCSIDevice2,
		data.SCSIDevice3,
		data.SCSIDevice4,
		data.SCSIDevice5,
		data.SCSIDevice6,
		data.SCSIDevice7,
		data.SCSIDevice8,
		data.SCSIDevice9,
		data.SCSIDevice10,
		data.SCSIDevice11,
		data.SCSIDevice12,
		data.SCSIDevice13,
	}

	log.Printf("[DEBUG] current size is %s", *storageDevices[index].Size)
	return *storageDevices[index].Size
}
