#include <iostream>
#include <ctime>
#include <chrono>

int main()
{
    // 获取当前系统时间
    std::chrono::system_clock::time_point now = std::chrono::system_clock::now();

    // 将当前时间转换为可读格式
    std::time_t now_c = std::chrono::system_clock::to_time_t(now);

    // 打印当前时间
    std::cout << "Current time is: " << std::ctime(&now_c) << std::endl;

    return 0;
}