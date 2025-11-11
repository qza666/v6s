package sysutils

import (
	"log"
	"os/exec"
	"os/user"
)

func AddV6Route(cidr string) {
	ensureRoot()

	delCmd := exec.Command("ip", "route", "del", "local", cidr, "dev", "lo")
	if err := delCmd.Run(); err != nil {
		log.Printf("尝试删除已存在的本地路由时出现错误：%v", err)
	}

	addCmd := exec.Command("ip", "route", "add", "local", cidr, "dev", "lo")
	if err := addCmd.Run(); err != nil {
		log.Fatalf("添加路由失败：%v", err)
	}
	log.Printf("已为本地接口添加路由 %s", cidr)
}

func SetV6Forwarding() {
	ensureRoot()

	if err := exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run(); err != nil {
		log.Fatalf("启用 IPv6 转发失败：%v", err)
	}
}

func SetIpNonLocalBind() {
	ensureRoot()

	if err := exec.Command("sysctl", "-w", "net.ipv6.ip_nonlocal_bind=1").Run(); err != nil {
		log.Fatalf("启用 IPv6 非本地绑定失败：%v", err)
	}
}

func ensureRoot() {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("获取当前用户失败：%v", err)
	}
	if currentUser.Uid != "0" {
		log.Fatal("请使用 root 权限运行该程序")
	}
}
