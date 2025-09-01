package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
)

type ZabbixRequest struct {
    JsonRPC string      `json:"jsonrpc"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params"`
    ID      int         `json:"id"`
}

type ZabbixResponse struct {
    JsonRPC string           `json:"jsonrpc"`
    Result  json.RawMessage  `json:"result"`
    Error   *ZabbixAPIError  `json:"error,omitempty"`
}

type ZabbixAPIError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    string `json:"data"`
}

func callZabbixAPI(apiURL, token, method string, params interface{}) (json.RawMessage, error) {
    req := ZabbixRequest{
        JsonRPC: "2.0",
        Method:  method,
        Params:  params,
        ID:      1,
    }

    body, _ := json.Marshal(req)
    request, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
    request.Header.Set("Content-Type", "application/json")
    request.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(request)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBody, _ := ioutil.ReadAll(resp.Body)

    var zResp ZabbixResponse
    if err := json.Unmarshal(respBody, &zResp); err != nil {
        return nil, err
    }

    if zResp.Error != nil {
        return nil, fmt.Errorf("Erro da API: %s", zResp.Error.Data)
    }

    return zResp.Result, nil
}

func main() {
    apiURL := "https://zabbix.pucrs.br/api_jsonrpc.php"
    token := "9575e71936057d9a6d663e2445d8d037e26d01c5de227bc31167f46fbe78d85c"

    params := map[string]interface{}{
        "name": "TestGroup",
    }

    result, err := callZabbixAPI(apiURL, token, "hostgroup.create", params)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Grupo criado:", string(result))
}
