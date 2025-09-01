package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type ZabbixRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
	Auth    string      `json:"auth,omitempty"`
}

type ZabbixResponse struct {
	Jsonrpc string           `json:"jsonrpc"`
	Result  json.RawMessage  `json:"result"`
	Error   *ZabbixAPIError  `json:"error,omitempty"`
}

type ZabbixAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func callZabbixAPI(apiURL, auth, method string, params interface{}) (json.RawMessage, error) {
	reqBody := ZabbixRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
		Auth:    auth,
	}

	body, _ := json.Marshal(reqBody)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var zResp ZabbixResponse
	if err := json.NewDecoder(resp.Body).Decode(&zResp); err != nil {
		return nil, err
	}

	if zResp.Error != nil {
		return nil, fmt.Errorf("API error %d: %s - %s", zResp.Error.Code, zResp.Error.Message, zResp.Error.Data)
	}

	return zResp.Result, nil
}

func main() {
	var (
		apiURL       string
		serverDNS    string
		token        string
		numHosts     int
		numItems     int
	)

	fmt.Print("URL do Zabbix (ex.: https://127.0.0.1/zabbix/api_jsonrpc.php): ")
	fmt.Scanln(&apiURL)
	fmt.Print("Zabbix Server (ex.: 127.0.0.1): ")
	fmt.Scanln(&serverDNS)
	fmt.Print("Token da API: ")
	fmt.Scanln(&token)
	fmt.Print("Número de hosts de teste: ")
	fmt.Scanln(&numHosts)
	fmt.Print("Número de itens por host: ")
	fmt.Scanln(&numItems)

	// 1. Criar grupo de testes
	groupParams := map[string]interface{}{
		"name": "PerformanceTestGroup",
	}
	res, err := callZabbixAPI(apiURL, token, "hostgroup.create", groupParams)
	if err != nil {
		log.Fatalf("Erro ao criar grupo: %v", err)
	}
	var groupResp map[string][]string
	json.Unmarshal(res, &groupResp)
	groupID := groupResp["groupids"][0]
	fmt.Println("Grupo de teste criado, ID:", groupID)

	// 2. Criar hosts e itens
	var wg sync.WaitGroup

	for i := 1; i <= numHosts; i++ {
		hostName := fmt.Sprintf("PerfTestHost-%d", i)
		hostParams := map[string]interface{}{
			"host": hostName,
			"interfaces": []map[string]interface{}{
				{
					"type": 1,
					"main": 1,
					"useip": 1,
					"ip": serverDNS,
					"dns": "",
					"port": "10050",
				},
			},
			"groups": []map[string]string{{"groupid": groupID}},
		}

		hRes, err := callZabbixAPI(apiURL, token, "host.create", hostParams)
		if err != nil {
			log.Fatalf("Erro ao criar host %s: %v", hostName, err)
		}
		var hResp map[string][]string
		json.Unmarshal(hRes, &hResp)
		hostID := hResp["hostids"][0]
		fmt.Println("Host criado:", hostName, "ID:", hostID)

		// Criar itens de teste e simular atualização
		for j := 1; j <= numItems; j++ {
			itemParams := map[string]interface{}{
				"name": fmt.Sprintf("PerfItem-%d", j),
				"key_": fmt.Sprintf("perf.test[%d]", j),
				"hostid": hostID,
				"type": 2,
				"value_type": 3,
				"delay": "1s",
			}
			_, err := callZabbixAPI(apiURL, token, "item.create", itemParams)
			if err != nil {
				log.Fatalf("Erro ao criar item para host %s: %v", hostName, err)
			}
		}

		// Subprocessos simulando envio de dados
		wg.Add(1)
		go func(hID string) {
			defer wg.Done()
			for k := 0; k < 50; k++ {
				// Simula envio de valor via zabbix trapper (no real sender here, just placeholder)
				fmt.Printf("[SIMULAÇÃO] Enviando dados para host %s...\n", hID)
				time.Sleep(100 * time.Millisecond)
			}
		}(hostID)
	}

	wg.Wait()
	fmt.Println("Teste de performance concluído.")
}
