import java.util.Date;

public class GetTime {
    public static void main(String[] args) {
        long timeInMillis = System.currentTimeMillis();

        Date date = new Date(timeInMillis);

        System.out.println("Current time is：" + date);
    }
}