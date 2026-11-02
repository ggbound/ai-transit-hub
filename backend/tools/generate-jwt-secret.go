
package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	// 生成 32 字节的随机密钥（256位）
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		panic(err)
	}

	// 使用 base64 编码输出
	encoded := base64.StdEncoding.EncodeToString(secret)
	
	fmt.Println("生成的 JWT_SECRET (64字符):")
	fmt.Println(encoded)
	fmt.Println()
	
	// 也生成一个更简短但足够安全的版本（URL安全）
	urlSafe := base64.RawURLEncoding.EncodeToString(secret)
	fmt.Println("URL安全版 (43字符):")
	fmt.Println(urlSafe)
	fmt.Println()
	
	fmt.Println("复制上面的密钥到您的 .env 文件中")
	fmt.Println("注意：请妥善保管此密钥，请勿泄露！")
	fmt.Println("如果修改密钥，所有已有的 token 将失效")
}
