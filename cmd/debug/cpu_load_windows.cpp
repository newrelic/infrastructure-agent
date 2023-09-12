#include <iostream>
#define _WIN32_WINNT 0x0602
#include <cstdint>
#include <ctime>
#include <iterator>
#include <locale>
#include <stdlib.h>
#include <windows.h>
// #include <processthreadsapi.h>

class Processor {};

uint64_t FromFileTime(const FILETIME &ft) {
  // ULARGE_INTEGER uli = {0};
  // uli.LowPart = ft.dwLowDateTime;
  // uli.HighPart = ft.dwHighDateTime;
  // return uli.QuadPart;
  return (((uint64_t)ft.dwHighDateTime) << 32) + (uint64_t)ft.dwLowDateTime;
  // float LOT = 0.0000001f;
  // float HIT = LOT * 4294967296.0f;
  // return HIT * (float)ft.dwHighDateTime + LOT * (float)ft.dwLowDateTime;
}

class Usage : public Processor {
public:
  // int now()
  // {
  //     FILETIME a0, a1, a2, b0, b1, b2;

  //     GetSystemTimes(&a0, &a1, &a2);
  //     SleepEx(250, false);
  //     GetSystemTimes(&b0, &b1, &b2);

  //     // attempt to get the actual value instead of the pointer and convert
  //     it to float/double/int float idle0 = a0; float idle1 = b0; float
  //     kernel0 = a1; float kernel1 = b1; float user0 = a2; float user1 = b2;

  //     float idl = idle1 - idle0;
  //     float ker = kernel0 - kernel1;
  //     float usr = user0 - user1;

  //     float cpu = (ker - idl + usr) * 100 / (ker + usr);

  //     return cpu;
  // }

  FILETIME a0, a1, a2;

  void utcExample() {
    // Example of the very popular RFC 3339 format UTC time
    std::time_t time = std::time({});
    char timeString[std::size("yyyy-mm-dd hh:mm:ss")];
    std::strftime(std::data(timeString), std::size(timeString), "%F %T",
                  std::gmtime(&time));
    std::string str(timeString);
    printf("| %s ", str);
    // return str;
  }

  void now() {
    FILETIME b0, b1, b2;

    // GetSystemTimes(&a0, &a1, &a2);
    SleepEx(250, false);
    bool err = GetSystemTimes(&b0, &b1, &b2);

    if (!err) {
      std::cerr << "ERROR IN GetSystemTimes !! Error code 0x" << std::hex
                << GetLastError() << std::endl;
    }

    printf("IDLE HIGH: %d, IDLE LOW: %d ", b0.dwHighDateTime, b0.dwLowDateTime);
    printf("KERN HIGH: %d, KERN LOW: %d ", b1.dwHighDateTime, b1.dwLowDateTime);
    printf("USER HIGH: %d, USER LOW: %d\n", b2.dwHighDateTime,
           b2.dwLowDateTime);

    uint64_t idle0 = FromFileTime(a0);
    uint64_t idle1 = FromFileTime(b0);
    uint64_t kernel0 = FromFileTime(a1);
    uint64_t kernel1 = FromFileTime(b1);
    uint64_t user0 = FromFileTime(a2);
    uint64_t user1 = FromFileTime(b2);
    uint64_t idl = idle1 - idle0;
    uint64_t ker = kernel1 - kernel0;
    uint64_t usr = user1 - user0;
    uint64_t sys0 = kernel0 - idle0;
    uint64_t use0 = user0 + sys0;
    uint64_t sys1 = kernel1 - idle1;
    uint64_t use1 = user1 + sys1;
    uint64_t sys = sys1 - sys0;
    uint64_t use = use1 - use0;
    uint64_t total = ker + usr + idl;
    uint64_t cpu_sys = ker - idl; // kernel1 - kernel0 - (idle1 - idle0) ==>
                                  // kernel1 - kernel0 - idle1 + idle0
    uint64_t cpu_use = usr + cpu_sys;

    utcExample();

    PROCESSOR_NUMBER res;
    GetCurrentProcessorNumberEx(&res);
    // printf(" GROUP: %d | NUMBER: %d |", res.Group, res.Number);
    // printf("| %10.4f | %10.4f | %10.4f | %10.4f | %10.4f |\n", use, usr, sys,
    // idl + ker, total); printf("| %20llu | %20llu | %20llu |\n", user1,
    // kernel1, idle1);

    float cpu_usage = ((float)use) * 100.0f / (float)total;
    float cpu_user = ((float)usr) * 100.0f / (float)total;
    float cpu_system = ((float)sys) * 100.0f / (float)total;
    float cpu_idle = ((float)idl) * 100.0f / (float)total;
    float cpu_kern = ((float)ker) * 100.0f / (float)total;
    float total_percent = cpu_user + cpu_kern + cpu_idle;
    float idle_percent = 100 - cpu_kern - cpu_user;

    // utcExample();
    // printf("| %10.4f | %10.4f | %10.4f | %10.4f | %10.4f |\n", cpu_usage,
    // cpu_user, cpu_system, idle_percent, total_percent);

    a0 = b0;
    a1 = b1;
    a2 = b2;

    // std::cout << cpu_usage << "|  " << cpu_user << "  |  " << cpu_system  <<
    // "  |" << std::endl;

    // return static_cast<float>(cpu);
  }
};

int main() {
  using namespace std;

  Usage Usage;

  // for (int i = 0; i < 10; i++)
  Usage.utcExample();
  // printf("| %10s | %10s | %10s | %10s | %10s |\n", "In Use", "User",
  // "System", "Idle", "TOTAL");
  printf("| User | Kernel | Idle |\n");
  while (true) {
    Usage.now();
  }

  cout << "\nFinished!\nPress any key to exit!\n";
  cin.clear();
  cin.get();
  return 0;
}
