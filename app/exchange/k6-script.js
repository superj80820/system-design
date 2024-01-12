import http from 'k6/http';
import { sleep } from 'k6';

export const options = {
  // A number specifying the number of VUs to run concurrently.
  vus: 10,
  // A string specifying the total duration of the test run.
  duration: '30s',

  // The following section contains configuration options for execution of this
  // test script in Grafana Cloud.
  //
  // See https://grafana.com/docs/grafana-cloud/k6/get-started/run-cloud-tests-from-the-cli/
  // to learn about authoring and running k6 test scripts in Grafana k6 Cloud.
  //
  // ext: {
  //   loadimpact: {
  //     // The ID of the project to which the test is assigned in the k6 Cloud UI.
  //     // By default tests are executed in default project.
  //     projectID: "",
  //     // The name of the test in the k6 Cloud UI.
  //     // Test runs with the same name will be grouped.
  //     name: "script.js"
  //   }
  // },

  // Uncomment this section to enable the use of Browser API in your tests.
  //
  // See https://grafana.com/docs/k6/latest/using-k6-browser/running-browser-tests/ to learn more
  // about using Browser API in your test scripts.
  //
  // scenarios: {
  //   // The scenario name appears in the result summary, tags, and so on.
  //   // You can give the scenario any name, as long as each name in the script is unique.
  //   ui: {
  //     // Executor is a mandatory parameter for browser-based tests.
  //     // Shared iterations in this case tells k6 to reuse VUs to execute iterations.
  //     //
  //     // See https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/ for other executor types.
  //     executor: 'shared-iterations',
  //     options: {
  //       browser: {
  //         // This is a mandatory parameter that instructs k6 to launch and
  //         // connect to a chromium-based browser, and use it to run UI-based
  //         // tests.
  //         type: 'chromium',
  //       },
  //     },
  //   },
  // }
};

// The function that defines VU logic.
//
// See https://grafana.com/docs/k6/latest/examples/get-started-with-k6/ to learn more
// about authoring k6 scripts.
//
export default function() {
  const url = 'http://localhost:9090/api/v1/orders';
  const randomVal = Math.floor(Math.random() * 100)
  const direction = Math.floor(Math.random() * 2)+1
  const payload = JSON.stringify({ "direction": direction, "price": 2.34+randomVal, "quantity": randomVal });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'user-id': '2'
    },
  };

  http.post(url, payload, params);
}

// export default function() {
//   const url = 'http://127.0.0.1/api/orders';
//   const randomVal = Math.floor(Math.random() * 100)
//   const direction = Math.floor(Math.random() * 2)
//   const directionMap = ["buy","sell"]
//   const payload = JSON.stringify({"productId":"BTC-USDT","side":directionMap[direction],"type":"limit","price":10+randomVal,"size":1+randomVal,"funds":(10+randomVal)*(1+randomVal)});

//   const params = {
//     headers: {
//       "Content-Type": "application/json;charset=UTF-8",
//       "Cookie": "JSESSIONID=9BBD64B1B3DED2880AF139FB82ECCB8A; accessToken=ff009c63-efb2-4c54-9b5f-9b88105522f4:9BBD64B1B3DED2880AF139FB82ECCB8A:35e8d276a558b1d6f96295fe01eb2381",
//     },
//   };

//   http.post(url, payload, params);
// }
