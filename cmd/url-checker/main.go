package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/sudosantos27/go-url-checker/internal/checker"
)

func main() {
	// 1. Definición de flags
	fileFlag := flag.String("file", "", "Ruta al archivo con la lista de URLs (requerido)")
	concurrencyFlag := flag.Int("concurrency", 5, "Número de workers concurrentes")

	flag.Parse()

	// 2. Validación de flags
	if *fileFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: El flag -file es obligatorio.")
		flag.Usage()
		os.Exit(1)
	}

	if *concurrencyFlag < 1 {
		fmt.Fprintln(os.Stderr, "Advertencia: -concurrency debe ser al menos 1. Usando 1.")
		*concurrencyFlag = 1
	}

	// 3. Lectura del archivo de URLs
	urls, err := readURLs(*fileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al leer el archivo: %v\n", err)
		os.Exit(1)
	}

	if len(urls) == 0 {
		fmt.Println("El archivo está vacío. No hay nada que procesar.")
		return
	}

	// 4. Llamada al paquete interno
	fmt.Printf("Iniciando chequeo con %d workers...\n", *concurrencyFlag)
	checker.Check(urls)
}

// readURLs lee el archivo línea por línea y devuelve un slice de strings.
func readURLs(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			urls = append(urls, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}
