package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
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

// sendValues executa o zabbix_sender para enviar valores a um host durante um tempo determinado.
func sendValues(serverDNS, hostName string, testDurationSec int, wg *sync.WaitGroup) {
	defer wg.Done()
	
	log.Printf("[INFO] Iniciando envio de valores para o host '%s' por %d segundos...", hostName, testDurationSec)
	
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(testDurationSec) * time.Second)
	sentValues := 0
	
	for time.Now().Before(endTime) {
		value := fmt.Sprintf("%d", sentValues+1)
		key := "perf.test[1]"
		
		cmd := exec.Command("zabbix_sender", "-z", serverDNS, "-s", hostName, "-k", key, "-o", value)
		
		err := cmd.Run()
		if err != nil {
			log.Printf("[ERRO] Falha ao enviar valor para '%s': %v", hostName, err)
		}
		
		sentValues++
		// Pequena pausa para simular a carga distribuída
		time.Sleep(10 * time.Millisecond)
	}
	log.Printf("[INFO] Envio de dados para o host '%s' concluído. Total de valores enviados: %d.", hostName, sentValues)
}

func main() {
	var (
		apiURL         string
		serverDNS      string
		token          string
		numHosts       int
		numItems       int
		testDurationSec int
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
	fmt.Print("Duração do teste em segundos: ")
	fmt.Scanln(&testDurationSec)
	
	// 1. Verificar/criar grupo de testes
	groupName := "PerformanceTestGroup"
	var groupID string
	
	// Tenta obter o grupo existente
	groupGetParams := map[string]interface{}{
		"output": "extend",
		"filter": map[string]string{
			"name": groupName,
		},
	}
	res, err := callZabbixAPI(apiURL, token, "hostgroup.get", groupGetParams)
	if err != nil {
		log.Fatalf("Erro ao buscar grupo: %v", err)
	}
	
	var existingGroups []map[string]interface{}
	json.Unmarshal(res, &existingGroups)
	
	if len(existingGroups) > 0 {
		groupID = existingGroups[0]["groupid"].(string)
		fmt.Println("Grupo de teste já existe, ID:", groupID)
	} else {
		// Se o grupo não existir, cria um novo
		groupCreateParams := map[string]interface{}{
			"name": groupName,
		}
		res, err := callZabbixAPI(apiURL, token, "hostgroup.create", groupCreateParams)
		if err != nil {
			log.Fatalf("Erro ao criar grupo: %v", err)
		}
		var groupCreateResp map[string][]string
		json.Unmarshal(res, &groupCreateResp)
		groupID = groupCreateResp["groupids"][0]
		fmt.Println("Grupo de teste criado, ID:", groupID)
	}
	
	hostNames := make([]string, 0, numHosts)

	// 2. Verificar/criar hosts e itens
	for i := 1; i <= numHosts; i++ {
		hostName := fmt.Sprintf("PerfTestHost-%d", i)
		var hostID string
		
		// Tenta obter o host existente
		hostGetParams := map[string]interface{}{
			"output": "extend",
			"filter": map[string]string{
				"host": hostName,
			},
		}
		hRes, err := callZabbixAPI(apiURL, token, "host.get", hostGetParams)
		if err != nil {
			log.Fatalf("Erro ao buscar host %s: %v", hostName, err)
		}
		
		var existingHosts []map[string]interface{}
		json.Unmarshal(hRes, &existingHosts)
		
		if len(existingHosts) > 0 {
			hostID = existingHosts[0]["hostid"].(string)
			fmt.Println("Host já existe:", hostName, "ID:", hostID)
		} else {
			// Se o host não existir, cria um novo
			hostCreateParams := map[string]interface{}{
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
			hRes, err := callZabbixAPI(apiURL, token, "host.create", hostCreateParams)
			if err != nil {
				log.Fatalf("Erro ao criar host %s: %v", hostName, err)
			}
			var hCreateResp map[string][]string
			json.Unmarshal(hRes, &hCreateResp)
			hostID = hCreateResp["hostids"][0]
			fmt.Println("Host criado:", hostName, "ID:", hostID)
		}
		
		hostNames = append(hostNames, hostName)
		
		// 3. Verificar/criar itens do tipo trapper para simular dados
		for j := 1; j <= numItems; j++ {
			itemKey := fmt.Sprintf("perf.test[%d]", j)
			
			// Tenta obter o item existente
			itemGetParams := map[string]interface{}{
				"output": "extend",
				"hostids": hostID,
				"search": map[string]string{
					"key_": itemKey,
				},
			}
			iRes, err := callZabbixAPI(apiURL, token, "item.get", itemGetParams)
			if err != nil {
				log.Fatalf("Erro ao buscar item com chave %s: %v", itemKey, err)
			}
			
			var existingItems []map[string]interface{}
			json.Unmarshal(iRes, &existingItems)
			
			if len(existingItems) == 0 {
				// Se o item não existir, cria um novo
				itemCreateParams := map[string]interface{}{
					"name":       fmt.Sprintf("PerfItem-%d", j),
					"key_":       itemKey,
					"hostid":     hostID,
					"type":       2, // Zabbix trapper
					"value_type": 0, // Numeric float
					"delay":      "0", // Não há polling, aguarda dados
				}
				_, err := callZabbixAPI(apiURL, token, "item.create", itemCreateParams)
				if err != nil {
					log.Fatalf("Erro ao criar item para host %s: %v", hostName, err)
				}
				fmt.Printf("  Item '%s' criado para o host '%s'\n", itemKey, hostName)
			} else {
				fmt.Printf("  Item '%s' já existe para o host '%s'\n", itemKey, hostName)
			}
		}
	}
	
	// 4. Enviar valores simultaneamente para cada host usando goroutines.
	fmt.Println("\nIniciando envio simultâneo de dados com zabbix_sender...")
	var wg sync.WaitGroup
	for _, hostName := range hostNames {
		wg.Add(1)
		go sendValues(serverDNS, hostName, testDurationSec, &wg)
	}

	wg.Wait()
	fmt.Println("\nTeste de performance concluído.")
}
