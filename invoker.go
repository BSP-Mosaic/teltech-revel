package revel

import (
	"reflect"

	"github.com/BSP-Mosaic/teltech-glog"
	"golang.org/x/net/websocket"
)

var (
	controllerType    = reflect.TypeOf(Controller{})
	controllerPtrType = reflect.TypeOf(&Controller{})
	websocketType     = reflect.TypeOf((*websocket.Conn)(nil))
)

func ActionInvoker(c *Controller, _ []Filter) {
	// Instantiate the method.
	methodValue := reflect.ValueOf(c.AppController).MethodByName(c.MethodType.Name)

	// Collect the values for the method's arguments.
	var methodArgs []reflect.Value
	for _, arg := range c.MethodType.Args {
		// If they accept a websocket connection, treat that arg specially.
		var boundArg reflect.Value
		if arg.Type == websocketType {
			boundArg = reflect.ValueOf(c.Websocket)
		} else {
			glog.V(1).Infoln("Binding:", arg.Name, "as", arg.Type)
			boundArg = Bind(c.Params, arg.Name, arg.Type)
		}
		methodArgs = append(methodArgs, boundArg)
	}

	var resultValue reflect.Value
	if methodValue.Type().IsVariadic() {
		resultValue = methodValue.CallSlice(methodArgs)[0]
	} else {
		resultValue = methodValue.Call(methodArgs)[0]
	}
	if resultValue.Kind() == reflect.Interface && !resultValue.IsNil() {
		c.Result = resultValue.Interface().(Result)
	}
}
