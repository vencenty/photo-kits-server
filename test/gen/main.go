package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "Yan3750346ct." // 你想设置的明文密码

	// 生成 bcrypt 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	// 输出哈希密码（存入数据库即可）
	fmt.Println(string(hashedPassword))
}
