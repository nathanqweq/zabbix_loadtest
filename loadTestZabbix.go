package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// ZabbixRequest define a estrutura de uma solicitação padrão para a API do Zabbix.
type ZabbixRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// ZabbixResponse define a estrutura de uma resposta da API do Zabbix.
type ZabbixResponse struct {
	Jsonrpc string           `json:"jsonrpc"`
	Result  json.RawMessage  `json:"result"`
	Error   *ZabbixAPIError  `json:"error,omitempty"`
}

// ZabbixAPIError define a estrutura de erro retornada pela API.
type ZabbixAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

// callZabbixAPI envia uma solicitação HTTP POST para a API do Zabbix com autenticação por token.
func callZabbixAPI(apiURL, token, method string, params interface{}) (json.RawMessage, error) {
	// Cria a requisição com o formato JSON esperado pela API.
	reqBody := ZabbixRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar o corpo da requisição: %v", err)
	}

	// Cria uma nova requisição HTTP para adicionar cabeçalhos.
	request, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar a requisição HTTP: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	// Cria e executa o cliente HTTP.
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar a requisição: %v", err)
	}
	defer resp.Body.Close()

	// Lê o corpo da resposta.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o corpo da resposta: %v", err)
	}

	// Decodifica a resposta JSON.
	var zResp ZabbixResponse
	if err := json.Unmarshal(respBody, &zResp); err != nil {
		return nil, fmt.Errorf("erro ao decodificar a resposta JSON: %v", err)
	}

	// Verifica se a resposta contém um erro da API.
	if zResp.Error != nil {
		return nil, fmt.Errorf("erro da API: %s - %s", zResp.Error.Message, zResp.Error.Data)
	}

	return zResp.Result, nil
}

func main() {
	var (
		apiURL    string
		serverDNS string
		token     string
		numHosts  int
		numItems  int
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
					"type":  1, // Agent
					"main":  1,
					"useip": 1,
					"ip":    serverDNS,
					"dns":   "",
					"port":  "10050",
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

		// Criar itens do tipo trapper para simular dados
		for j := 1; j <= numItems; j++ {
			itemParams := map[string]interface{}{
				"name":       fmt.Sprintf("PerfItem-%d", j),
				"key_":       fmt.Sprintf("perf.test[%d]", j),
				"hostid":     hostID,
				"type":       2, // Zabbix trapper
				"value_type": 0, // Numeric float
				"delay":      "0", // Não há polling, aguarda dados
			}
			_, err := callZabbixAPI(apiURL, token, "item.create", itemParams)
			if err != nil {
				log.Fatalf("Erro ao criar item para host %s: %v", hostName, err)
			}
		}

		// A seção de subprocessos foi removida, pois não é possível simular
		// o zabbix_sender a partir deste código Go sem o executável.
		// A lógica original era apenas um placeholder.

		// Para testar o código, você precisaria executar o zabbix_sender
		// manualmente, apontando para os hosts e itens criados.
		// Ex: zabbix_sender -z <serverDNS> -s <hostName> -k <key_> -o <valor>
	}

	wg.Wait()
	fmt.Println("Teste de performance concluído.")
}
