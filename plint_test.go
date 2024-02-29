package main

import "testing"

var hdr = `
int add(int, int);
int sub(int, int);
void nothing(void);
`

var srcOk = `
int add(int lhs, int rhs)
{
    return lhs + rhs;
}

int sub(int lhs, int rhs)
{
    return lhs + rhs;
}

void nothing(void)
{
}
`

var srcNotInOrder = `
int add(int lhs, int rhs)
{
    return lhs + rhs;
}

void nothing(void)
{
}

int sub(int lhs, int rhs)
{
    return lhs + rhs;
}
`

func TestSourceOk(t *testing.T) {
	err := Validate("header.h", []byte(hdr), "source.c", []byte(srcOk))
	if err != nil {
		t.Fatalf("%s\n", err)
	}
}

func TestSourceNotInOrder(t *testing.T) {
	err := Validate("header.h", []byte(hdr), "source.c", []byte(srcNotInOrder))
	if err == nil {
		t.Fatal("Expected an error\n")
	}
	t.Log(err)
}

var hdrOnlyOk = `
int add(int, int);
int sub(int, int);
void nothing(void);

int add(int lhs, int rhs)
{
    return lhs + rhs;
}

int sub(int lhs, int rhs)
{
    return lhs + rhs;
}

void nothing(void)
{
}
`
var hdrOnlyNotInOrder = `
int add(int, int);
int sub(int, int);
void nothing(void);

int add(int lhs, int rhs)
{
    return lhs + rhs;
}

void nothing(void)
{
}

int sub(int lhs, int rhs)
{
    return lhs + rhs;
}
`

func TestHeaderOnlyOk(t *testing.T) {
	err := Validate("fname.h", []byte(hdrOnlyOk), "fname.h", []byte(hdrOnlyOk))
	if err != nil {
		t.Fatalf("%s\n", err)
	}
}

func TestHeaderOnlyNotInOrder(t *testing.T) {
	err := Validate("fname.h", []byte(hdrOnlyNotInOrder), "fname.h", []byte(hdrOnlyNotInOrder))
	if err == nil {
		t.Fatal("Expected an error\n")
	}
	t.Log(err)
}

var runOutOfFnDeclarations = `
int add(int, int);
int sub(int, int);

int add(int lhs, int rhs)
{
    return lhs + rhs;
}

int sub(int lhs, int rhs)
{
    return lhs + rhs;
}

void nothing(void)
{
}
`

var runOutOfFnDefinitions = `
int add(int, int);
int sub(int, int);
void nothing(void);

int add(int lhs, int rhs)
{
    return lhs + rhs;
}

int sub(int lhs, int rhs)
{
    return lhs + rhs;
}
`

func TestRunOutOfFnDeclarations(t *testing.T) {
	err := Validate("fname.h", []byte(runOutOfFnDeclarations), "fname.h", []byte(runOutOfFnDeclarations))
	if err == nil {
		t.Fatal("Expected an error\n")
	}
	t.Log(err)
}

func TestRunOutOfFnDefinitions(t *testing.T) {
	err := Validate("fname.h", []byte(runOutOfFnDefinitions), "fname.h", []byte(runOutOfFnDefinitions))
	if err == nil {
		t.Fatal("Expected an error\n")
	}
	t.Log(err)
}
