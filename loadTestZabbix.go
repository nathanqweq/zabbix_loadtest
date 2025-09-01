package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// Structs para API Zabbix
type ZabbixRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Auth    string      `json:"auth"`
	ID      int         `json:"id"`
}

type ZabbixResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

func callZabbixAPI(url, token, method string, params interface{}) json.RawMessage {
	req := ZabbixRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		Auth:    token,
		ID:      1,
	}
	body, _ := json.Marshal(req)
	resp, err := exec.Command("curl", "-s", "-X", "POST", "-H", "Content-Type: application/json", "-d", string(body), url).Output()
	if err != nil {
		log.Fatalf("Erro ao chamar API: %v", err)
	}
	var zResp ZabbixResponse
	if err := json.Unmarshal(resp, &zResp); err != nil {
		log.Fatalf("Erro ao ler resposta da API: %v", err)
	}
	if zResp.Error != nil {
		log.Fatalf("Erro da API: %v", zResp.Error.Data)
	}
	return zResp.Result
}

func createGroup(url, token, name string) string {
	params := map[string]string{"name": name}
	res := callZabbixAPI(url, token, "hostgroup.create", params)
	var parsed struct {
		GroupIDs []string `json:"groupids"`
	}
	json.Unmarshal(res, &parsed)
	return parsed.GroupIDs[0]
}

func createHost(url, token, name, groupID string) string {
	params := map[string]interface{}{
		"host": name,
		"groups": []map[string]string{
			{"groupid": groupID},
		},
		"interfaces": []map[string]interface{}{
			{
				"type":  1, // Agent
				"main":  1,
				"useip": 1,
				"ip":    "127.0.0.1",
				"dns":   "",
				"port":  "10050",
			},
		},
	}
	res := callZabbixAPI(url, token, "host.create", params)
	var parsed struct {
		HostIDs []string `json:"hostids"`
	}
	json.Unmarshal(res, &parsed)
	return parsed.HostIDs[0]
}

func createItem(url, token, hostID, key string) string {
	params := map[string]interface{}{
		"name":       key,
		"key_":       key,
		"hostid":     hostID,
		"type":       2, // trapper
		"value_type": 3, // numeric unsigned
	}
	res := callZabbixAPI(url, token, "item.create", params)
	var parsed struct {
		ItemIDs []string `json:"itemids"`
	}
	json.Unmarshal(res, &parsed)
	return parsed.ItemIDs[0]
}

func sendValue(zabbixServer, host, key, value string) {
	cmd := exec.Command("zabbix_sender", "-z", zabbixServer, "-s", host, "-k", key, "-o", value)
	_ = cmd.Run()
}

func main() {
	var url, token, zabbixServer string
	var hosts, itens int

	fmt.Print("URL do Zabbix (ex.: http://127.0.0.1/zabbix/api_jsonrpc.php): ")
	fmt.Scanln(&url)
	fmt.Print("Zabbix Server (ex.: 127.0.0.1): ")
	fmt.Scanln(&zabbixServer)
	fmt.Print("Token da API: ")
	fmt.Scanln(&token)
	fmt.Print("Número de hosts de teste: ")
	fmt.Scanln(&hosts)
	fmt.Print("Número de itens por host: ")
	fmt.Scanln(&itens)

	fmt.Println("Criando grupo de teste...")
	groupID := createGroup(url, token, "LoadTestGroup")

	hostNames := []string{}
	for i := 1; i <= hosts; i++ {
		name := fmt.Sprintf("LoadTestHost_%d", i)
		fmt.Printf("Criando host: %s\n", name)
		hID := createHost(url, token, name, groupID)
		for j := 1; j <= itens; j++ {
			key := fmt.Sprintf("loadtest.item.%d", j)
			createItem(url, token, hID, key)
		}
		hostNames = append(hostNames, name)
	}

	fmt.Println("Iniciando geração de carga...\nCTRL+C para parar.")
	var wg sync.WaitGroup
	for _, h := range hostNames {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			for {
				for j := 1; j <= itens; j++ {
					key := fmt.Sprintf("loadtest.item.%d", j)
					sendValue(zabbixServer, host, key, fmt.Sprintf("%d", time.Now().Unix()))
				}
				time.Sleep(1 * time.Second)
			}
		}(h)
	}

	wg.Wait()
}
